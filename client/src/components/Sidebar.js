import React, { useState } from 'react';
import { Plus, Search, X, Lock, MessageCircle, ChevronRight } from 'lucide-react';
import Avatar from './Avator';

// Ensure this utility file exists and is correctly implemented
import { formatSystemMessage } from '../utils/messageUtils';

// 🌟 Added 'currentUser' to the destructured props
const Sidebar = ({ rooms, activeRoomId, onRoomSelect, onNewRoom, onViewExplore, isMobile, onClose, currentUser }) => {
    const [searchQuery, setSearchQuery] = useState('');

    const filteredRooms = rooms.filter(room =>
        room.name.toLowerCase().includes(searchQuery.toLowerCase())
    );

    const hasRooms = rooms.length > 0;
    const hasFiltered = filteredRooms.length > 0;

    return (
        <div className={`sidebar ${isMobile ? 'sidebar--mobile' : ''}`}>
            {/* Header */}
            <div className="sidebar__header">
                <div className="sidebar__header-top">
                    <div className="sidebar__title">
                        <MessageCircle size={28}/>
                        <h2>ChatHub</h2>
                    </div>
                    <div className="sidebar__header-actions">
                        <button onClick={onNewRoom} className="icon-button">
                            <Plus />
                        </button>
                        {isMobile && (
                            <button onClick={onClose} className="icon-button icon-button--mobile-only">
                                <X />
                            </button>
                        )}
                    </div>
                </div>

                {/* Search */}
                <div className="search-bar">
                    <Search className="search-bar__icon" />
                    <input
                        type="text"
                        placeholder="Search conversations..."
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="search-bar__input"
                    />
                </div>
            </div>

            {/* Room List */}
            <div className="sidebar__joined-rooms">
                {hasFiltered ? (
                    filteredRooms.map(room => {
                        
                        const formattedLastMessage = currentUser 
                            ? formatSystemMessage(room.lastMessage, room.lastSenderId, currentUser.username) 
                            : room.lastMessage;
                        
                        return (
                            <div
                                key={room.id}
                                onClick={() => {
                                    onRoomSelect(room.id);
                                    if (isMobile) onClose();
                                }}
                                className={`room-item ${activeRoomId === room.id ? 'room-item--active' : ''}`}
                            >
                                <div className="room-item__inner">
                                    <Avatar size="lg">{room.avatar}</Avatar>
                                    <div className="room-item__content">
                                        <div className="room-item__header">
                                            <h3 className="room-item__name">
                                                <span>{room.name}</span>
                                                {room.isPrivate && <Lock className="room-item__lock-icon" />}
                                            </h3>
                                            <span className="room-item__time" style={{ color: room.unread > 0 ? "var(--primary-color)" : "var(--gray-300)"}}>{room.lastMessageTime}</span>
                                        </div>
                                        {/* 🌟 Using the formatted message */}
                                        <p className="room-item__message">{formattedLastMessage}</p> 
                                        <div className="room-item__meta">
                                            <span className="room-item__meta-text">{room.members} members</span>
                                            {room.unread > 0 && (
                                                <span className="room-item__unread">{room.unread}</span>
                                            )}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        );
                    }) // End of map function
                ) : (
                    <div className="sidebar__empty">
                        <MessageCircle className="sidebar__empty-icon" />
                        {hasRooms ? (
                            <>
                                <h3 className="sidebar__empty-title">No matching rooms</h3>
                                <p className="sidebar__empty-text">Try a different search term</p>
                            </>
                        ) : (
                            <>
                                <h3 className="sidebar__empty-title">No rooms yet</h3>
                                <p className="sidebar__empty-text">
                                    Create a new room or join existing ones to start chatting
                                </p>
                            </>
                        )}
                    </div>
                )}
            </div>
            
            <div className="sidebar__explore-rooms-container">
                <div className="explore-item" onClick={onViewExplore} >
                    <span className="explore-item__text">Discover New Rooms</span>
                    <ChevronRight className="explore-item__icon" />
                </div>
                <p className="explore-item__description">
                    Browse through all public chat rooms and find new communities to engage with.
                </p>
            </div>

        </div>
    );
};

export default Sidebar;