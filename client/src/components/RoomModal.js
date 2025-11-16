import React, { useState } from 'react';
import { X, Hash, Info, Loader } from 'lucide-react';
import { createRoom } from '../utils/api';
import '../styles/Modal.css'; 

const Spinner = () => (
    <Loader size={18} className="spinner" style={{ marginRight: '8px' }} />
);

const CreateRoomModal = ({ isOpen, onClose, onRoomCreated }) => {
    const [name, setName] = useState('');
    const [description, setDescription] = useState('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);

    if (!isOpen) return null;

    const handleSubmit = async (e) => {
        e.preventDefault();
        if (!name.trim()) return;

        setLoading(true);
        setError(null);

        try {
            const newRoom = await createRoom({ name, description });
            onRoomCreated(newRoom);
            setName('');
            setDescription('');
            onClose();
        } catch (err) {
            setError('Failed to create room: ' + err.message);
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="modal-backdrop" onClick={onClose} role="dialog" aria-modal="true" aria-labelledby="modal-title">
            {/* Stop clicks inside the modal content from closing it */}
            <div className="modal-content" onClick={e => e.stopPropagation()}>
                
                <div className="modal-header">
                    <h3 id="modal-title">Create New Room</h3>
                    <button className="icon-button modal-close-btn" onClick={onClose} aria-label="Close modal">
                        <X size={20} />
                    </button>
                </div>

                <div className="modal-body">
                    {/* Error message designed to stand out slightly */}
                    {error && (
                        <div className="alert-error flex">
                            <Info size={16} />
                            <p>{error}</p>
                        </div>
                    )}

                    <form onSubmit={handleSubmit} className="modal-form flex-cl">
                        
                        {/* Room Name Input */}
                        <div className="input-grp flex-cl align-start">
                            <label htmlFor="room-name" className="input-label">
                                <Hash size={14} style={{ marginRight: '4px' }}/> Room Name
                            </label>
                            <input
                                id="room-name"
                                type="text"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                placeholder="e.g. Project Alpha"
                                required
                                className="text-input"
                                maxLength={50}
                            />
                            <p className="small-text text-hint">This will be the main name for your group chat.</p>
                        </div>

                        {/* Description Input */}
                        <div className="input-grp flex-cl align-start">
                            <label htmlFor="room-description" className="input-label">
                                <Info size={14} style={{ marginRight: '4px' }}/> Description (Optional)
                            </label>
                            <textarea
                                id="room-description"
                                value={description}
                                onChange={(e) => setDescription(e.target.value)}
                                placeholder="What is this room for? (max 200 characters)"
                                rows={3}
                                className="text-input"
                                maxLength={200}
                            />
                        </div>

                        {/* Submission Button */}
                        <button 
                            type="submit" 
                            className="button button--primary modal-submit-btn" 
                            disabled={loading || !name.trim()}
                        >
                            {loading && <Spinner />}
                            {loading ? 'Creating Room...' : 'Create Room'}
                        </button>

                    </form>
                </div>
            </div>
        </div>
    );
};

export default CreateRoomModal;