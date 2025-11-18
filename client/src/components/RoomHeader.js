import { Info, ArrowLeft, Phone, Video, Lock } from 'lucide-react';

import Avatar from "./Avator";

const RoomHeader = ({ room, onInfoClick, onBackClick, isMobile }) => {
  return (
    <div className="room-header">
      <div className="room-header__left">
        {isMobile && (
          <button onClick={onBackClick} className="icon-button">
            <ArrowLeft />
          </button>
        )}
        <Avatar size="md" username={room?.name}>{room?.avatar}</Avatar>
        <div>
          <h2 className="room-header__name">
            <span>{room?.name}</span>
            {room?.isPrivate && <Lock className="room-item__lock-icon" />}
          </h2>
          <p className="room-header__status">{room?.members} members • 5 online</p>
        </div>
      </div>

      <div className="room-header__actions">
        <button className="icon-button icon-button--desktop-only">
          <Phone />
        </button>
        <button className="icon-button icon-button--desktop-only">
          <Video />
        </button>
        <button onClick={onInfoClick} className="icon-button">
          <Info />
        </button>
      </div>
    </div>
  );
};

export default RoomHeader;