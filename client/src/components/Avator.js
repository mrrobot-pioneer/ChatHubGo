const Avatar = ({ children, size = 'md', status, className = '' }) => {
  const statusClass = status ? `avatar__status--${status}` : '';

  return (
    <div className={`avatar avatar--${size} ${className}`}>
      <div className="avatar__body">
        {children}
      </div>
      {status && (
        <div className={`avatar__status ${statusClass}`} />
      )}
    </div>
  );
};

export default Avatar;