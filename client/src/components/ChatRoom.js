import React, { useState, useEffect, useRef } from 'react';
import { Send, Paperclip, Smile, Hash } from 'lucide-react';
import RoomHeader from './RoomHeader';
import Message from './Message';
import '../styles/Chat.css';

const ChatRoom = ({ room, messages, currentUser, onSendMessage, isMobile, onBackClick, onInfoClick, hasRooms}) => {
    const [inputMessage, setInputMessage] = useState('');
    const messagesEndRef = useRef(null);

    // Scroll to bottom on new messages
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

    const handleTextareaChange = (e) => {
    setInputMessage(e.target.value);
    e.target.style.height = 'auto';
    e.target.style.height = `${e.target.scrollHeight}px`;
    };

    // Case 1: User has no rooms at all
    if (!hasRooms) {
    return (
        <div className="chat-room-placeholder">
        <div className="chat-room-placeholder__content">
            <Hash className="chat-room-placeholder__icon" />
            <h3 className="chat-room-placeholder__title">No rooms yet</h3>
            <p className="chat-room-placeholder__subtitle">
                Create a new room or join existing ones to start chatting
            </p>
        </div>
        </div>
    );
    }

    // Case 2: User has rooms but hasn’t selected any
    if (!room) {
    return (
        <div className="chat-room-placeholder">
        <div className="chat-room-placeholder__content">
            <Hash className="chat-room-placeholder__icon" />
            <h3 className="chat-room-placeholder__title">Select a conversation</h3>
            <p className="chat-room-placeholder__subtitle">
            Choose a room from the sidebar to start chatting
            </p>
        </div>
        </div>
    );
    }

    // Case 3: Active room
    return (
    <div className="chat-room">
        <RoomHeader
        room={room}
        onInfoClick={onInfoClick}
        onBackClick={onBackClick}
        isMobile={isMobile}
        />

        <div className="chat-room__messages">
        {messages.map(msg => (
            <Message key={msg.id} message={msg} isOwn={msg.senderId === currentUser.id} currentUser={currentUser}/>
        ))}
        <div ref={messagesEndRef} />
        </div>

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

export default ChatRoom;