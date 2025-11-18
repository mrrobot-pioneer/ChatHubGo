import React, { useState, useEffect } from 'react';
import { X, UserPlus, LogOut, UserMinus, Trash2, Loader } from 'lucide-react';

import Avatar from "./Avator";
import { getRoomMembers, removeMember, deleteRoom, leaveRoom } from '../utils/api';

const RoomInfoPanel = ({ room, isOpen, onClose, currentUser, isMobile, onRoomDeleted, onRoomLeft }) => {
  const [members, setMembers] = useState([]);
  const [currentUserRole, setCurrentUserRole] = useState(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState(null);

  // Fetch members when panel opens or room changes
  useEffect(() => {
    if (!isOpen || !room?.id) return;

    const fetchMembers = async () => {
      try {
        setIsLoading(true);
        setError(null);
        const fetchedMembers = await getRoomMembers(room.id);
        setMembers(fetchedMembers || []);

        // Find current user's role
        const currentMember = fetchedMembers.find(m => m.id === currentUser.id);
        setCurrentUserRole(currentMember?.role || null);
      } catch (err) {
        console.error("Failed to fetch room members:", err);
        setError("Failed to load members");
      } finally {
        setIsLoading(false);
      }
    };

    fetchMembers();
  }, [isOpen, room?.id, currentUser.id]);

  if (!isOpen) return null;

  const handleRemoveMember = async (memberId, memberName) => {
    if (!window.confirm(`Remove ${memberName} from this room?`)) return;

    try {
      await removeMember(room.id, memberId);
      setMembers(prev => prev.filter(m => m.id !== memberId));
    } catch (err) {
      console.error("Failed to remove member:", err);
      alert(`Failed to remove member: ${err.message}`);
    }
  };

  const handleDeleteRoom = async () => {
    if (!window.confirm(`Are you sure you want to delete "${room.name}"? This action cannot be undone.`)) return;

    try {
      await deleteRoom(room.id);
      onClose();
      if (onRoomDeleted) onRoomDeleted(room.id);
    } catch (err) {
      console.error("Failed to delete room:", err);
      alert(`Failed to delete room: ${err.message}`);
    }
  };

  const handleLeaveRoom = async () => {
    if (!window.confirm(`Leave "${room.name}"?`)) return;

    try {
      await leaveRoom(room.id);
      onClose();
      if (onRoomLeft) onRoomLeft(room.id);
    } catch (err) {
      console.error("Failed to leave room:", err);
      alert(`Failed to leave room: ${err.message}`);
    }
  };

  const isAdmin = currentUserRole === 'admin';

  return (
    <div className={`room-info ${isMobile ? 'room-info--mobile' : 'room-info--desktop'}`}>
      {/* Header */}
      <div className="room-info__header">
        <h3 className="room-info__title">Room Info</h3>
        <button onClick={onClose} className="icon-button">
          <X />
        </button>
      </div>

      <div className="room-info__scrollable">
        {/* Room Details */}
        <div className="room-info__details">
          <Avatar size="xl" className="room-info__avatar" username={room?.name}>{room?.avatar}</Avatar>
          <h2 className="room-info__name">{room?.name}</h2>
          <p className="room-info__description">{room?.description}</p>
          <div className="room-info__add-members">
            <button className="button button--primary">
              <UserPlus className="button__icon" />
              <span className="button__text">Add Members</span>
            </button>
          </div>
        </div>

        {/* Members */}
        <div className="room-info__members">
          <h4 className="room-info__members-title">
            Members ({members?.length || 0})
          </h4>

          {isLoading ? (
            <div className="room-info__loading">
              <Loader className="spinner" size={24} />
              <p>Loading members...</p>
            </div>
          ) : error ? (
            <div className="room-info__error">
              <p>{error}</p>
            </div>
          ) : (
            <div className="room-info__members-list">
              {members?.map(member => (
                <div key={member.id} className="member-item">
                  <Avatar size="md" username={member.username}>{member.username.charAt(0).toUpperCase()}</Avatar>
                  <div className="member-item__content">
                    <p className="member-item__name">{member.username}</p>
                    <p className="member-item__role">{member.role}</p>
                  </div>
                  {member.id === currentUser.id && (
                    <span className="member-item__you">You</span>
                  )}
                  {isAdmin && member.id !== currentUser.id && (
                    <button
                      onClick={() => handleRemoveMember(member.id, member.username)}
                      className="icon-button icon-button--danger"
                      title="Remove member"
                    >
                      <UserMinus size={18} />
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Admin Actions */}
        {isAdmin && (
          <div className="room-info__admin-actions">
            <button onClick={handleDeleteRoom} className="button button--danger">
              <Trash2 className="button__icon" />
              <span className="button__text">Delete Room</span>
            </button>
          </div>
        )}

        {/* Leave Room */}
        <div className="room-info__leave">
          <button onClick={handleLeaveRoom} className="button button--leave">
            <LogOut className="button__icon" />
            <span className="button__text">Leave Room</span>
          </button>
        </div>
      </div>
    </div>
  );
};

export default RoomInfoPanel;
