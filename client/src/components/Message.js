import { Check, CheckCheck } from 'lucide-react';

import Avatar from './Avator';

const Message = ({ message, isOwn }) => {
  return (
    <div className={`message ${isOwn ? 'message--own' : 'message--guest'}`}>
      {!isOwn && <Avatar size="sm">{message.avatar}</Avatar>}
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
            message.read ?
              <CheckCheck className="message__read-receipt" /> :
              <Check className="message__sent-receipt" />
          )}
        </div>
      </div>
    </div>
  );
};

export default Message;