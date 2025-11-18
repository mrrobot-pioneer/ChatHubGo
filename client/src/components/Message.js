import { Check, CheckCheck } from 'lucide-react';
import Avatar from './Avator';

import { formatSystemMessage } from '../utils/messageUtils';

const Message = ({ message, isOwn, currentUser }) => {
    const isSystemMessage = message.senderId === 1;

    let displayText = message.text;
    if (isSystemMessage) {
        displayText = formatSystemMessage(message.text, message.senderId, currentUser.username);
    }

    if (isSystemMessage) {
        // System Message Rendering (uses displayText)
        return (
            <div className="message-item message-item--system">
                <span className="message-item__system-text">{displayText}</span>
            </div>
        );
    }

    // Determine message status for visual feedback
    const isOptimistic = message.isOptimistic === true;
    const messageClass = isOptimistic ? 'message--pending' : '';

    // Regular Message Rendering
    return (
        <div className={`message ${isOwn ? 'message--own' : 'message--guest'} ${messageClass}`}>
            {!isOwn && <Avatar size="sm" username={message.sender}>{message.avatar}</Avatar>}
            <div className="message__content">
                {!isOwn && (
                    <span className="message__sender">~{message.sender}</span>
                )}
                <div className="message__bubble">
                    <p className="message__text">{message.text}</p>
                </div>
                <div className="message__meta">
                    <span className="message__timestamp">{message.timestamp}</span>
                    {isOwn && (
                        isOptimistic ? (
                            <Check className="message__sent-receipt message__sent-receipt--pending" />
                        ) : message.read ? (
                            <CheckCheck className="message__read-receipt" />
                        ) : (
                            <Check className="message__sent-receipt" />
                        )
                    )}
                </div>
            </div>
        </div>
    );
};

export default Message;