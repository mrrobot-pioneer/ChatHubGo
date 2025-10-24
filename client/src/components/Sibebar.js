import React, { useState } from 'react';
import { Plus, Search, X, Lock} from 'lucide-react';

import Avatar from './Avator';

const Sidebar = ({ rooms, activeRoomId, onRoomSelect, onNewRoom, isMobile, onClose }) => {
  const [searchQuery, setSearchQuery] = useState('');

  const filteredRooms = rooms.filter(room =>
    room.name.toLowerCase().includes(searchQuery.toLowerCase())
  );

  return (
    <div className={`sidebar ${isMobile ? 'sidebar--mobile' : ''}`}>
      {/* Header */}
      <div className="sidebar__header">
        <div className="sidebar__header-top">
          <h2 className="sidebar__title">ChatHub</h2>
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
      <div className="sidebar__room-list">
        {filteredRooms.map(room => (
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
                  <span className="room-item__time">{room.lastMessageTime}</span>
                </div>
                <p className="room-item__message">{room.lastMessage}</p>
                <div className="room-item__meta">
                  <span className="room-item__meta-text">{room.members} members</span>
                  {room.unread > 0 && (
                    <span className="room-item__unread">
                      {room.unread}
                    </span>
                  )}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};

export default Sidebar;