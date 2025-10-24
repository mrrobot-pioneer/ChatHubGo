import { X, Image as ImageIcon,  Bell, UserPlus, LogOut } from 'lucide-react';

import Avatar from "./Avator";

const RoomInfoPanel = ({ room, members, isOpen, onClose, currentUser, isMobile }) => {
  if (!isOpen) return null;

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
          <Avatar size="xl" className="room-info__avatar">{room?.avatar}</Avatar>
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
          <div className="room-info__members-list">
            {members?.map(member => (
              <div key={member.id} className="member-item">
                <Avatar size="md" status={member.status}>{member.avatar}</Avatar>
                <div className="member-item__content">
                  <p className="member-item__name">{member.username}</p>
                  <p className="member-item__role">{member.role}</p>
                </div>
                {member.id === currentUser.id && (
                  <span className="member-item__you">You</span>
                )}
              </div>
            ))}
          </div>
        </div>

        {/* Leave Room */}
        <div className="room-info__leave">
          <button className="button button--leave">
            <LogOut className="button__icon" />
            <span className="button__text">Leave Room</span>
          </button>
        </div>
      </div>
    </div>
  );
};

export default RoomInfoPanel;