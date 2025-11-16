import React, { useState, useEffect, useRef, useCallback } from 'react';
import {useNavigate} from 'react-router-dom';

import Sidebar from '../components/Sidebar';
import ChatRoom from '../components/ChatRoom';
import RoomInfoPanel from '../components/RoomInfoPanel';
import RoomModal from '../components/RoomModal';
import ExploreRoomsList from '../components/ExploreRoomsList';

import { getRooms, getRoomMessages, joinRoom } from '../utils/api';
import { getToken, getUser, removeAuth } from '../utils/auth';

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
  const [showCreateModal, setShowCreateModal] = useState(false); 
  const ws = useRef(null);
  const [currentView, setCurrentView] = useState('joined');

  useEffect(() => {
    const handleResize = () => {
      const mobile = window.innerWidth < 768;
      setIsMobile(mobile);
      // If resizing to desktop, always close room info if it was open on mobile
      if (!mobile) {
        setShowRoomInfo(false);
      }
    };

    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  const navigate = useNavigate();

  // --- Utility for Handling Expired Tokens ---
  const handleAuthError = (error) => {
    if (error.message.includes("Invalid token") || error.message.includes("is not authenticated")) {        
        // 1. Clear token/user data from localStorage
        removeAuth();

        // 2. Redirect to the login page
        navigate('/login');
        
        // Close WebSocket if it was open on a bad token
        if (ws.current && ws.current.readyState === WebSocket.OPEN) {
            ws.current.close();
        }
        return true; 
    }
    return false;
  };

  // --- Initial Data Loading & User Info ---
  useEffect(() => {
    // 1. Get Logged in User Data
    const loggedUser = getUser();
    if (loggedUser) {
        setCurrentUser(loggedUser);
    } else {
        navigate('/login');
        return;
    }

    // 2. Fetch Rooms
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
  }, []); // Empty dependency array means this runs once on mount

  // --- WebSocket Setup ---
  useEffect(() => {
    const token = getToken();
    if (!token) return;

    const WS_URL = `ws://localhost:8080/ws?token=${token}`; 

    ws.current = new WebSocket(WS_URL);

    ws.current.onopen = () => {
      console.log('WebSocket connected');

      if (activeRoomId) {
        ws.current.send(JSON.stringify({ type: 'joinRoom', room_id: activeRoomId }));
      }
    };

    ws.current.onmessage = (event) => {
      const data = JSON.parse(event.data);
      console.log('WS Message received:', data);

      if (data.type === 'roomMessage' && data.Message) {
        // Map the Go Message struct to your React Message format
        const newMessage = {
            id: data.Message.id,
            senderId: data.Message.sender_id,
            sender: data.Message.sender,
            text: data.Message.text,
            timestamp: new Date(data.Message.timestamp).toLocaleTimeString(),
            avatar: data.Message.avatar,
            read: true,
        };

        setMessages(prev => {
            // Only add if it belongs to the active room
            if (activeRoomId === data.Message.room_id) {
                return [...prev, newMessage];
            }
            return prev;
        });

        // OPTIONAL: Update room's last message in the rooms list
        setRooms(prevRooms => prevRooms.map(r => r.id === data.Message.room_id ? { ...r, lastMessage: data.Message.text, lastMessageTime: 'Just now' } : r));
      }
    };

    ws.current.onclose = () => {
      console.log('WebSocket closed');
    };

    ws.current.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    // --- CRITICAL CLEANUP FUNCTION ---
    return () => {
        // If the connection is open or connecting, close it gracefully.
        // This runs when the component unmounts OR when React Strict Mode cleans up the effect.
        if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
            ws.close(1000, "Component unmount/cleanup"); // Use 1000 for normal closure
            console.log("WebSocket cleanup: closed connection.");
        }
    };
  }, [activeRoomId]);

  // --- Room Selection and Message Fetching ---
  const fetchMessages = useCallback(async (roomId) => {
    try {
      const fetchedMessages = await getRoomMessages(roomId);
      // Map the Go Message struct fields to your frontend fields
      const formattedMessages = fetchedMessages.map(m => ({
        id: m.id,
        senderId: m.sender_id,
        sender: m.sender,
        text: m.text,
        timestamp: new Date(m.timestamp).toLocaleTimeString(),
        avatar: m.avatar,
        read: true,
      }));
      setMessages(formattedMessages);

      setActiveRoomId(roomId);

    } catch (error) {
      console.error("Failed to fetch messages:", error);
      setMessages([]);
    }
  }, []);


  // --- Event Handlers ---
  const handleSendMessage = (text) => {
    if (!activeRoomId || !ws.current || ws.current.readyState !== WebSocket.OPEN) {
      console.error('WebSocket not ready or no active room');
      return;
    }

    const messagePayload = {
      type: 'sendMessage',
      room_id: activeRoomId,
      content: text,
    };
    
    ws.current.send(JSON.stringify(messagePayload));
  };
  
  const handleRoomCreated = (newRoom) => {
    // Add the new room to the state instantly
    setRooms(prevRooms => [newRoom, ...prevRooms]);
    // Optionally set it as active
    setActiveRoomId(newRoom.id);
    fetchMessages(newRoom.id);
  };

  const activeRoom = rooms.find(r => r.id === activeRoomId);
  const currentLoggedUser = currentUser;
  
  // Recalculate isMobile for RoomInfoPanel prop
  // This is a bit redundant but ensures the prop is correct
  // We can simplify this logic.
  const isMobileView = window.innerWidth < 768;


  const handleRoomSelect = (roomId) => {
    setActiveRoomId(roomId);
    setShowSidebar(false);

    if (isMobile) {
      // On mobile: hide room info (space is limited)
      setShowRoomInfo(false);
    } else {
      // On desktop: automatically show room info when a room is selected
      setShowRoomInfo(true);
    }
  };

  const handleBackToRooms = () => {
    setActiveRoomId(null);
    setShowSidebar(true);
  };
  
  const handleToggleRoomInfo = () => {
    setShowRoomInfo(prev => !prev);
  }

  // Mobile/Desktop layout logic
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
      // 1. API call to join the room, returns the full room object
      const newRoom = await joinRoom(roomId); 

      // 2. Update the joined rooms list by appending the new room
      setRooms(prevRooms => [newRoom, ...prevRooms]);

      // 3. Switch back to the joined view and select the new room
      setActiveRoomId(roomId);
      setCurrentView('joined');
      
      // OPTIONAL: If on mobile, close the sidebar
      if (isMobile) {
        setShowSidebar(false);
      }

    } catch (error) {
      console.error("Failed to join room:", error);
      if (handleAuthError(error)) return;
      
      // Handle other join errors (e.g., already a member)
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
    
    // Default: 'joined' view
    return (
      <Sidebar
        rooms={rooms}
        activeRoomId={activeRoomId}
        onRoomSelect={handleRoomSelect}
        onNewRoom={() => setShowCreateModal(true)}
        onViewExplore={handleViewExplore} // New prop name used here
        isMobile={isMobile}
        onClose={() => setShowSidebar(false)}
        currentUser={currentLoggedUser}
      />
    );
  };

  return (
    <div className="chat-app">
      
      {/* Sidebar - Desktop always visible, Mobile conditional */}
      {showLeftPanel && (
        renderSidebar()
      )}

      {/* Chat Room - Desktop always visible, Mobile conditional */}
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

      {/* Room Info Panel - Desktop sidebar, Mobile overlay */}
      {showRightPanel && activeRoom && (
        <RoomInfoPanel
          room={activeRoom}
          members={[]}
          isOpen={showRoomInfo}
          onClose={() => setShowRoomInfo(false)}
          currentUser={currentLoggedUser}
          isMobile={isMobile}
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
