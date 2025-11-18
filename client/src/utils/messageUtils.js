export const formatSystemMessage = (text, senderId, currentUsername) => {
    if (senderId !== 1) {
        return text;
    }

    const joinMatch = text.match(/^(.+?) joined this room at .*?\.$/);
    const createMatch = text.match(/^(.+?) created this room at .*?\.$/);

    const match = joinMatch || createMatch;
    
    if (match) {
        const systemUsername = match[1];
        if (systemUsername === currentUsername) {
            return text.replace(systemUsername, 'You');
        }
    }
    
    return text;
};


export const formatTime = (date) => {
  return date.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
    hour12: true
  });
};


export const getAvatarColor = (str) => {
  if (!str) return 'hsl(200, 15%, 75%)'; 

  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }

  const hue = Math.abs(hash % 360);

  const saturation = 25 + (Math.abs(hash) % 10);
  const lightness = 70 + (Math.abs(hash >> 8) % 10);

  return `hsl(${hue}, ${saturation}%, ${lightness}%)`;
};