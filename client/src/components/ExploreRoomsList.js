
import React, { useState, useEffect } from 'react';
import { ArrowLeft, Loader, Globe, Plus, AlertCircle } from 'lucide-react';
import Avatar from './Avator';
import { getAllRooms } from '../utils/api';

// New prop needed: onBack to return to the joined view, onJoin to handle joining a room
const ExploreRoomsList = ({ onBack, onJoin }) => {
  const [exploreRooms, setExploreRooms] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);

  // Fetch rooms only when this component is mounted
  useEffect(() => {
    const fetchRooms = async () => {
      try {
        setIsLoading(true);
        setError(null);
        // getAllRooms fetches rooms the user is NOT in
        const rooms = await getAllRooms(); 
        setExploreRooms(rooms);
      } catch (err) {
        console.error("Failed to fetch explorable rooms:", err);
        setError("Failed to load rooms. Please try again.");
      } finally {
        setIsLoading(false);
      }
    };
    fetchRooms();
  }, []);

  const renderRooms = () => {
    if (isLoading) {
      return (
        <div className="explore-list__status">
          <Loader className="spinner explore-list__spinner" />
          <p>Loading rooms...</p>
        </div>
      );
    }

    if (error) {
      return (
        <div className="error-message" style={{marginTop: "5rem"}}>
          <AlertCircle size={16}/>
          <p>{error}</p>
        </div>
      );
    }

    if (exploreRooms.length === 0) {
      return (
        <div className="explore-list__status">
          <Globe className="explore-list__icon" />
          <h3 className="explore-list__title">No Public Rooms Found</h3>
          <p className="explore-list__text">Looks like you've joined everything, or no public rooms exist yet!</p>
        </div>
      );
    }

    return exploreRooms.map(room => (
      <div key={room.id} className="explore-room-item">
        <div className="explore-room-item__info">
          <Avatar size="md" username={room.name}>{room.avatar}</Avatar>
          <div className="explore-room-item__content">
            <h3 className="explore-room-item__name">
              {room.name} 
            </h3>
            <p className="explore-room-item__description">{room.description || "No description provided."}</p>
            <span className="explore-room-item__members">{room.members} members</span>
          </div>
        </div>
        <button 
          onClick={() => onJoin(room.id)} 
          className="button button--primary button--join"
        >
          <Plus size={16}/> Join
        </button>
      </div>
    ));
  };

  return (
    <div className="sidebar explore-view">
      
      {/* Header for Explore View */}
      <div className="sidebar__header explore-header">
        <div className="explore-header__top">
          <button onClick={onBack} className="icon-button icon-button--back">
            <ArrowLeft />
          </button>
          <h2 className="explore-header__title">Explore Rooms</h2>
        </div>
      </div>

      {/* Explore Rooms List (Scrollable) */}
      <div className="explore-list__container">
        {renderRooms()}
      </div>
    </div>
  );
};

export default ExploreRoomsList;