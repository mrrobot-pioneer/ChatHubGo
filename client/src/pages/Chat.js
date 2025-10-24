import React, { useState, useEffect, useRef } from 'react';
import { Send, Paperclip, Smile, Hash } from 'lucide-react';

import Sidebar from '../components/Sibebar';
import Message from '../components/Message';
import RoomHeader from '../components/RoomHeader';
import RoomInfoPanel from '../components/RoomInfoPanel';

import "../styles/Chat.css"

// Mock Data
const mockUser = { id: 1, username: 'John Doe', avatar: 'JD' };

const mockRooms = [
  { id: 1, name: 'Design Team', description: 'UI/UX discussions', lastMessage: 'Great work on the mockups!', lastMessageTime: '2m ago', unread: 3, isPrivate: false, members: 12, avatar: '🎨' },
  { id: 2, name: 'Dev Squad', description: 'Development updates', lastMessage: 'Pushed to production', lastMessageTime: '15m ago', unread: 0, isPrivate: false, members: 8, avatar: '💻' },
  { id: 3, name: 'Marketing', description: 'Campaign strategies', lastMessage: 'Meeting at 3pm', lastMessageTime: '1h ago', unread: 1, isPrivate: false, members: 5, avatar: '📢' },
  { id: 4, name: 'Sarah Wilson', description: null, lastMessage: 'Thanks for the update', lastMessageTime: '2h ago', unread: 0, isPrivate: true, members: 2, avatar: 'SW' },
];

const mockMessages = [
  { id: 1, senderId: 2, sender: 'Alice Cooper', text: 'Hey everyone! How are we doing with the new design?', timestamp: '10:30 AM', avatar: 'AC', read: true },
  { id: 2, senderId: 1, sender: 'John Doe', text: 'Making good progress! Just finishing up the mobile views.', timestamp: '10:32 AM', avatar: 'JD', read: true },
  { id: 3, senderId: 3, sender: 'Bob Smith', text: 'I have some feedback on the color scheme', timestamp: '10:35 AM', avatar: 'BS', read: true },
  { id: 4, senderId: 2, sender: 'Alice Cooper', text: 'Sure! Let me know your thoughts', timestamp: '10:36 AM', avatar: 'AC', read: true },
  { id: 5, senderId: 1, sender: 'John Doe', text: 'Great work on the mockups! The new layout looks fantastic.', timestamp: '10:40 AM', avatar: 'JD', read: false },
];

const mockRoomMembers = [
  { id: 1, username: 'John Doe', avatar: 'JD', role: 'admin', status: 'online' },
  { id: 2, username: 'Alice Cooper', avatar: 'AC', role: 'member', status: 'online' },
  { id: 3, username: 'Bob Smith', avatar: 'BS', role: 'member', status: 'away' },
  { id: 4, username: 'Carol White', avatar: 'CW', role: 'member', status: 'offline' },
  { id: 5, username: 'David Brown', avatar: 'DB', role: 'member', status: 'online' },
];

// Room Header Component


// Chat Room Component
const ChatRoom = ({ room, messages, currentUser, onSendMessage, isMobile, onBackClick, onInfoClick }) => {
  const [inputMessage, setInputMessage] = useState('');
  const messagesEndRef = useRef(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSend = () => {
    if (inputMessage.trim()) {
      onSendMessage(inputMessage);
      setInputMessage('');
    }
  };

  const handleKeyPress = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };
  
  // Auto-resize textarea
  const handleTextareaChange = (e) => {
    setInputMessage(e.target.value);
    e.target.style.height = 'auto'; // Reset height
    e.target.style.height = `${e.target.scrollHeight}px`; // Set to scroll height
  };

  if (!room) {
    return (
      <div className="chat-room-placeholder">
        <div className="chat-room-placeholder__content">
          <Hash className="chat-room-placeholder__icon" />
          <h3 className="chat-room-placeholder__title">Select a conversation</h3>
          <p className="chat-room-placeholder__subtitle">Choose a room from the sidebar to start chatting</p>
        </div>
      </div>
    );
  }

  return (
    <div className="chat-room">
      <RoomHeader room={room} onInfoClick={onInfoClick} onBackClick={onBackClick} isMobile={isMobile} />

      {/* Messages */}
      <div className="chat-room__messages">
        {messages.map(msg => (
          <Message key={msg.id} message={msg} isOwn={msg.senderId === currentUser.id} />
        ))}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="chat-room__input-area">
        <div className="chat-room__input-inner">
          <button className="icon-button">
            <Paperclip />
          </button>
          <div className="chat-room__input-wrapper">
            <textarea
              value={inputMessage}
              onChange={handleTextareaChange}
              onKeyPress={handleKeyPress}
              placeholder="Type a message..."
              rows={1}
              className="chat-room__textarea"
            />
            <button className="chat-room__smiley-button">
              <Smile />
            </button>
          </div>
          <button
            onClick={handleSend}
            disabled={!inputMessage.trim()}
            className="send-button"
          >
            <Send />
          </button>
        </div>
      </div>
    </div>
  );
};


// Main App Component
export default function Chat() {
  const [activeRoomId, setActiveRoomId] = useState(null);
  const [showSidebar, setShowSidebar] = useState(false);
  const [showRoomInfo, setShowRoomInfo] = useState(false);
  const [isMobile, setIsMobile] = useState(window.innerWidth < 768);

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
  
  // Recalculate isMobile for RoomInfoPanel prop
  // This is a bit redundant but ensures the prop is correct
  // We can simplify this logic.
  const isMobileView = window.innerWidth < 768;

  const activeRoom = mockRooms.find(r => r.id === activeRoomId);

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

  const handleSendMessage = (text) => {
    console.log('Sending message:', text);
    // Here you would add the new message to the mockMessages state
  };

  const handleNewRoom = () => {
    console.log('Creating new room');
  };
  
  const handleToggleRoomInfo = () => {
    setShowRoomInfo(prev => !prev);
  }

  // Mobile/Desktop layout logic
  const showLeftPanel = !isMobile || (isMobile && !activeRoomId);
  const showMiddlePanel = !isMobile || (isMobile && activeRoomId);
  const showRightPanel = showRoomInfo && (!isMobile || (isMobile && activeRoomId));

  return (
    <div className="chat-app">
      
      {/* Sidebar - Desktop always visible, Mobile conditional */}
      {showLeftPanel && (
        <Sidebar
          rooms={mockRooms}
          activeRoomId={activeRoomId}
          onRoomSelect={handleRoomSelect}
          onNewRoom={handleNewRoom}
          isMobile={isMobile}
          onClose={() => setShowSidebar(false)}
        />
      )}

      {/* Chat Room - Desktop always visible, Mobile conditional */}
      {showMiddlePanel && (
        <ChatRoom
          room={activeRoom}
          messages={mockMessages}
          currentUser={mockUser}
          onSendMessage={handleSendMessage}
          isMobile={isMobile}
          onBackClick={handleBackToRooms}
          onInfoClick={handleToggleRoomInfo}
        />
      )}

      {/* Room Info Panel - Desktop sidebar, Mobile overlay */}
      {showRightPanel && (
        <RoomInfoPanel
          room={activeRoom}
          members={mockRoomMembers}
          isOpen={showRoomInfo}
          onClose={() => setShowRoomInfo(false)}
          currentUser={mockUser}
          isMobile={isMobile}
        />
      )}
    </div>
  );
}
