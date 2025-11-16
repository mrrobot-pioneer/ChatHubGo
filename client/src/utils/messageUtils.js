
/**
 * Formats a message or lastMessage string for display, substituting 
 * the username with "You" if it's a system message from the current user.
 * * @param {string} text The raw message text (e.g., "Steve joined the room...")
 * @param {number} senderId The ID of the message sender (1 for System)
 * @param {string} currentUsername The username of the currently logged-in user
 * @returns {string} The formatted message text.
 */
export const formatSystemMessage = (text, senderId, currentUsername) => {
    // Check if it's a System message (System ID is 1)
    if (senderId !== 1) {
        return text;
    }

    // Regexes to match the system message patterns
    // Matches: "Username [joined/created] the room at .*."
    const joinMatch = text.match(/^(.+?) joined this room at .*?\.$/);
    const createMatch = text.match(/^(.+?) created this room at .*?\.$/);

    const match = joinMatch || createMatch;
    
    if (match) {
        const systemUsername = match[1];
        // If the system username matches the current user, substitute with 'You'
        if (systemUsername === currentUsername) {
            return text.replace(systemUsername, 'You');
        }
    }
    
    return text;
};