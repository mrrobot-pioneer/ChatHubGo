import React, { useState, useEffect, useRef, useCallback } from 'react';
import {useNavigate} from 'react-router-dom';

import Sidebar from '../components/Sidebar';
import ChatRoom from '../components/ChatRoom';
import RoomInfoPanel from '../components/RoomInfoPanel';
import RoomModal from '../components/RoomModal';
import ExploreRoomsList from '../components/ExploreRoomsList';

import { getRooms, getRoomMessages, joinRoom, markRoomAsRead } from '../utils/api';
import { getToken, getUser, removeAuth } from '../utils/auth';
import { formatTime } from '../utils/messageUtils';

import "../styles/Chat.css"


// Main App Component
export default function Chat() {
  const [activeRoomId, setActiveRoomId] = useState(null);
  const [showSidebar, setShowSidebar] = useState(false);
  const [showRoomInfo, setShowRoomInfo] = useState(false);
  const [isMobile, setIsMobile] = useState(window.innerWidth < 768);
  const [currentUser, setCurrentUser] = useState({ id: 0, username: 'User', avatar: 'U' });
  const [rooms, setRooms] = useState([]);
  const [messages, setMessages] = useState([]);
  const messagesCache = useRef({}); 
  const [showCreateModal, setShowCreateModal] = useState(false);
  const ws = useRef(null);
  const activeRoomIdRef = useRef(activeRoomId); 
  const [currentView, setCurrentView] = useState('joined');

  useEffect(() => {
    activeRoomIdRef.current = activeRoomId;
  }, [activeRoomId]);

  useEffect(() => {
    const handleResize = () => {
      const mobile = window.innerWidth < 768;
      setIsMobile(mobile);
      if (!mobile) {
        setShowRoomInfo(false);
      }
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  const navigate = useNavigate();

  const handleAuthError = (error) => {
    if (error.message.includes("Invalid token") || error.message.includes("is not authenticated")) {        
        removeAuth();

        navigate('/login');
        
        if (ws.current && ws.current.readyState === WebSocket.OPEN) {
            ws.current.close();
        }
        return true; 
    }
    return false;
  };

  // --- Initial Data Loading & User Info ---
  useEffect(() => {
    const loggedUser = getUser();
    if (loggedUser) {
        setCurrentUser(loggedUser);
    } else {
        navigate('/login');
        return;
    }

    const fetchRooms = async () => {
      try {
        const fetchedRooms = await getRooms();
        setRooms(fetchedRooms || []);
      } catch (error) {
        console.error("Failed to fetch rooms:", error);

        if (handleAuthError(error)) {
            return; 
        }
      }
    };
    fetchRooms();
  }, []); 

  // --- WebSocket Setup (Persistent Connection) ---
  useEffect(() => {
    const token = getToken();
    if (!token) return;

    const WS_URL = `ws://localhost:8080/ws?token=${token}`;

    ws.current = new WebSocket(WS_URL);

    ws.current.onopen = () => {
      if (activeRoomIdRef.current) {
        ws.current.send(JSON.stringify({ type: 'joinRoom', room_id: activeRoomIdRef.current }));
      }
    };

    ws.current.onmessage = (event) => {
      const data = JSON.parse(event.data);

      if (data.type === 'roomMessage') {
        const newMessage = {
            id: data.id,
            roomId: data.room_id,
            senderId: data.sender_id,
            sender: data.sender,
            text: data.text,
            timestamp: formatTime(new Date(data.timestamp)),
            avatar: data.avatar,
            read: true,
            isOptimistic: false,
        };

        const roomId = newMessage.roomId;
        const currentRoomMessages = messagesCache.current[roomId] || [];
        const isOwnMessage = newMessage.senderId === currentUser.id;

        let updatedRoomMessages;
        if (isOwnMessage) {
          const filtered = currentRoomMessages.filter(m =>
            !(m.isOptimistic && m.text === newMessage.text && m.senderId === currentUser.id)
          );
          const alreadyExists = filtered.some(m => m.id === newMessage.id);
          updatedRoomMessages = alreadyExists ? filtered : [...filtered, newMessage];
        } else {
          const alreadyExists = currentRoomMessages.some(m => m.id === newMessage.id);
          updatedRoomMessages = alreadyExists ? currentRoomMessages : [...currentRoomMessages, newMessage];
        }

        messagesCache.current[roomId] = updatedRoomMessages;

        if (activeRoomIdRef.current === roomId) {
          setMessages(updatedRoomMessages);
        }

        setRooms(prevRooms => {
          const isCurrentlyActiveRoom = activeRoomIdRef.current === roomId;
          const isOwnMessage = newMessage.senderId === currentUser.id;

          const updatedRooms = prevRooms.map(r => {
            if (r.id === roomId) {
              const shouldIncrementUnread = !isCurrentlyActiveRoom && !isOwnMessage;
              return {
                ...r,
                lastMessage: newMessage.text,
                lastMessageTime: 'Just now',
                lastSenderId: newMessage.senderId,
                unread: shouldIncrementUnread ? (r.unread || 0) + 1 : r.unread
              };
            }
            return r;
          });

          const roomWithMessage = updatedRooms.find(r => r.id === roomId);
          const otherRooms = updatedRooms.filter(r => r.id !== roomId);

          return roomWithMessage ? [roomWithMessage, ...otherRooms] : updatedRooms;
        });
      } else if (data.type === 'error') {
        console.error('WebSocket error:', data.content);
        if (activeRoomIdRef.current) {
          const filtered = messages.filter(m => !m.isOptimistic);
          messagesCache.current[activeRoomIdRef.current] = filtered;
          setMessages(filtered);
        }
      }
    };

    ws.current.onclose = (event) => {
      if (event.code !== 1000) {
        setTimeout(() => {
          const token = getToken();
          if (!token) return;

          const WS_URL = `ws://localhost:8080/ws?token=${token}`;
          ws.current = new WebSocket(WS_URL);

          ws.current.onopen = () => {
            if (activeRoomIdRef.current) {
              ws.current.send(JSON.stringify({ type: 'joinRoom', room_id: activeRoomIdRef.current }));
            }
          };

          ws.current.onmessage = (event) => {
            const data = JSON.parse(event.data);

            if (data.type === 'roomMessage') {
              const newMessage = {
                  id: data.id,
                  roomId: data.room_id,
                  senderId: data.sender_id,
                  sender: data.sender,
                  text: data.text,
                  timestamp: formatTime(new Date(data.timestamp)),
                  avatar: data.avatar,
                  read: true,
                  isOptimistic: false,
              };

              const roomId = newMessage.roomId;
              const currentRoomMessages = messagesCache.current[roomId] || [];
              const isOwnMessage = newMessage.senderId === currentUser.id;

              let updatedRoomMessages;
              if (isOwnMessage) {
                const filtered = currentRoomMessages.filter(m =>
                  !(m.isOptimistic && m.text === newMessage.text && m.senderId === currentUser.id)
                );
                const alreadyExists = filtered.some(m => m.id === newMessage.id);
                updatedRoomMessages = alreadyExists ? filtered : [...filtered, newMessage];
              } else {
                const alreadyExists = currentRoomMessages.some(m => m.id === newMessage.id);
                updatedRoomMessages = alreadyExists ? currentRoomMessages : [...currentRoomMessages, newMessage];
              }

              messagesCache.current[roomId] = updatedRoomMessages;

              if (activeRoomIdRef.current === roomId) {
                setMessages(updatedRoomMessages);
              }

              setRooms(prevRooms => prevRooms.map(r =>
                  r.id === roomId
                      ? { ...r, lastMessage: newMessage.text, lastMessageTime: 'Just now', lastSenderId: newMessage.senderId }
                      : r
              ));
            } else if (data.type === 'error') {
              console.error('WebSocket error from server:', data.content);
              if (activeRoomIdRef.current) {
                const filtered = messages.filter(m => !m.isOptimistic);
                messagesCache.current[activeRoomIdRef.current] = filtered;
                setMessages(filtered);
              }
            }
          };

          ws.current.onerror = (error) => {
            console.error('WebSocket error:', error);
            setMessages(prev => prev.filter(m => !m.isOptimistic));
          };

          ws.current.onclose = (event) => {
          };
        }, 3000);
      }
    };

    ws.current.onerror = (error) => {
      console.error('WebSocket error:', error);
      setMessages(prev => prev.filter(m => !m.isOptimistic));
    };

    return () => {
        if (ws.current && (ws.current.readyState === WebSocket.OPEN || ws.current.readyState === WebSocket.CONNECTING)) {
            ws.current.close(1000, "Component unmount");
        }
    };
  }, []);

  useEffect(() => {
    if (activeRoomId && ws.current && ws.current.readyState === WebSocket.OPEN) {
      ws.current.send(JSON.stringify({ type: 'joinRoom', room_id: activeRoomId }));
    }
  }, [activeRoomId]);

  const fetchMessages = useCallback(async (roomId) => {
    try {
      const fetchedMessages = await getRoomMessages(roomId);
      const formattedMessages = fetchedMessages.map(m => ({
        id: m.id,
        senderId: m.sender_id,
        sender: m.sender,
        text: m.text,
        timestamp: new Date(m.timestamp).toLocaleTimeString(),
        avatar: m.avatar,
        read: true,
        isOptimistic: false, 
      }));
      setMessages(formattedMessages);

      messagesCache.current[roomId] = formattedMessages;

      try {
        await markRoomAsRead(roomId);
        setRooms(prevRooms => prevRooms.map(r =>
          r.id === roomId ? { ...r, unread: 0 } : r
        ));
      } catch (error) {
        console.error("Failed to mark room as read:", error);
      }

      setActiveRoomId(roomId);

    } catch (error) {
      console.error("Failed to fetch messages:", error);
      setMessages([]);
      messagesCache.current[roomId] = [];
    }
  }, [setRooms]);


  // --- Event Handlers ---
  const handleSendMessage = (text) => {
    if (!activeRoomId || !ws.current || ws.current.readyState !== WebSocket.OPEN) {
      console.error('WebSocket not ready or no active room');
      return;
    }

    const optimisticMessage = {
      id: `temp-${Date.now()}`, 
      senderId: currentUser.id,
      sender: currentUser.username,
      text: text,
      timestamp: new Date().toLocaleTimeString(),
      avatar: currentUser.avatar,
      read: false, 
      isOptimistic: true,
    };

    setMessages(prev => [...prev, optimisticMessage]);

    const messagePayload = {
      type: 'sendMessage',
      room_id: activeRoomId,
      content: text,
    };

    ws.current.send(JSON.stringify(messagePayload));
  };
  
  const handleRoomCreated = (newRoom) => {
    setRooms(prevRooms => [newRoom, ...prevRooms]);
    setActiveRoomId(newRoom.id);
    fetchMessages(newRoom.id);
  };

  const activeRoom = rooms.find(r => r.id === activeRoomId);
  const currentLoggedUser = currentUser;
  

  const handleRoomSelect = (roomId) => {
    fetchMessages(roomId);
    setShowSidebar(false);

    if (isMobile) {
      setShowRoomInfo(false);
    } else {
      setShowRoomInfo(true);
    }
  };

  const handleBackToRooms = () => {
    setActiveRoomId(null);
    setShowSidebar(true);
  };
  
  const handleToggleRoomInfo = () => {
    setShowRoomInfo(prev => !prev);
  };

  const handleRoomDeleted = (roomId) => {
    setRooms(prevRooms => prevRooms.filter(r => r.id !== roomId));
    setActiveRoomId(null);
    setShowRoomInfo(false);
    if (isMobile) {
      setShowSidebar(true);
    }
  };

  const handleRoomLeft = (roomId) => {
    setRooms(prevRooms => prevRooms.filter(r => r.id !== roomId));
    setActiveRoomId(null);
    setShowRoomInfo(false);
    if (isMobile) {
      setShowSidebar(true);
    }
  };

  const handleLogout = () => {
    // Close WebSocket connection
    if (ws.current && ws.current.readyState === WebSocket.OPEN) {
      ws.current.close(1000, "User logged out");
    }

    // Clear authentication data
    removeAuth();

    // Navigate to login page
    navigate('/login');
  };

  const showLeftPanel = !isMobile || (isMobile && !activeRoomId);
  const showMiddlePanel = !isMobile || (isMobile && activeRoomId);
  const showRightPanel = showRoomInfo && (!isMobile || (isMobile && activeRoomId));

  const handleViewExplore = () => {
    setCurrentView('explore');
  };

  const handleBackToJoined = () => {
    setCurrentView('joined');
  };

const handleJoinRoom = async (roomId) => {
    try {
      const newRoom = await joinRoom(roomId); 

      setRooms(prevRooms => [newRoom, ...prevRooms]);

      setActiveRoomId(roomId);
      setCurrentView('joined');
      
      if (isMobile) {
        setShowSidebar(false);
      }

    } catch (error) {
      console.error("Failed to join room:", error);
      if (handleAuthError(error)) return;
      
      alert(`Could not join room: ${error.message}`);
    }
  };

  const renderSidebar = () => {
    if (currentView === 'explore') {
      return (
        <ExploreRoomsList 
          onBack={handleBackToJoined} 
          onJoin={handleJoinRoom}
        />
      );
    }
    
    return (
      <Sidebar
        rooms={rooms}
        activeRoomId={activeRoomId}
        onRoomSelect={handleRoomSelect}
        onNewRoom={() => setShowCreateModal(true)}
        onViewExplore={handleViewExplore}
        isMobile={isMobile}
        onClose={() => setShowSidebar(false)}
        currentUser={currentLoggedUser}
        onLogout={handleLogout}
      />
    );
  };

  return (
    <div className="chat-app">
      
      {showLeftPanel && (
        renderSidebar()
      )}

      {showMiddlePanel &&  (
        <ChatRoom
          room={activeRoom}
          messages={messages}
          currentUser={currentLoggedUser}
          onSendMessage={handleSendMessage}
          isMobile={isMobile}
          onBackClick={handleBackToRooms}
          onInfoClick={handleToggleRoomInfo}
          hasRooms={rooms && rooms.length > 0}
        />
      )}

      {showRightPanel && activeRoom && (
        <RoomInfoPanel
          room={activeRoom}
          isOpen={showRoomInfo}
          onClose={() => setShowRoomInfo(false)}
          currentUser={currentLoggedUser}
          isMobile={isMobile}
          onRoomDeleted={handleRoomDeleted}
          onRoomLeft={handleRoomLeft}
        />
      )}

      <RoomModal 
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onRoomCreated={handleRoomCreated}
      />
    </div>
  );
}
