import { getAvatarColor } from '../utils/messageUtils';

const Avatar = ({ children, size = 'md', status, className = '', username = '' }) => {
  const statusClass = status ? `avatar__status--${status}` : '';
  const backgroundColor = getAvatarColor(username);

  return (
    <div className={`avatar avatar--${size} ${className}`}>
      <div className="avatar__body" style={{ backgroundColor }}>
        {children}
      </div>
      {status && (
        <div className={`avatar__status ${statusClass}`} />
      )}
    </div>
  );
};

export default Avatar;