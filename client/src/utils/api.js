import { getToken } from './auth';

const API_BASE_URL = "http://localhost:8080/api";

/**
 * Utility function to read and throw a specific error from the server response.
 * @param {Response} response - The failed fetch Response object.
 * @param {string} defaultMessage - The fallback message if no body is available.
 */
async function handleApiError(response, defaultMessage) {
    let errorText = defaultMessage;
    try {
        // Attempt to read the specific error message from the response body
        const errorData = await response.text();
        // Use the server's message if it's not empty, otherwise use the default
        errorText = errorData || defaultMessage;
    } catch {
        // Ignore if response body is unreadable (e.g., CORS error, network issue)
    }
    // Throw a new error with the most relevant text
    throw new Error(errorText);
}


// Utility to register a new user
export async function register({ username, email, password }) {
  const res = await fetch(`${API_BASE_URL}/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, email, password }),
  });
    
  if (!res.ok) {
        // 🌟 FIX: Use the new helper to get the server's error message
        await handleApiError(res, "Registration failed. Please check your details.");
    }
  return res.json();
}

// Utility to login a user
export async function login({ username, password }) {
  const res = await fetch(`${API_BASE_URL}/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });

  if (!res.ok) {
        // 🌟 FIX: Use the new helper to get the server's error message
        await handleApiError(res, "Login failed. Invalid username or password.");
    }
  return res.json();
}

// Protected fetch utility
async function protectedFetch(url, options = {}) {
  const token = getToken();
  if (!token) {
    // Specific error message for frontend handling (redirect)
    throw new Error('User is not authenticated'); 
  }

  const defaultHeaders = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`,
  };

  const response = await fetch(API_BASE_URL + url, {
    ...options,
    headers: {
      ...defaultHeaders,
      ...options.headers,
    },
  });

  if (!response.ok) {
    // This existing logic already handles server errors well
    // It's functionally identical to handleApiError now, but kept separate for clarity
    // in handling the protected logic.
    let errorText = 'An unknown error occurred';
    try {
      const errorData = await response.text();
      errorText = errorData || errorText;
    } catch {
    // Ignore if response body is not readable
      }
    throw new Error(errorText);
 }
  return response.json();
}

// API to create a new room
export const createRoom = (roomDetails) => {
  return protectedFetch('/rooms', {
    method: 'POST',
    body: JSON.stringify(roomDetails),
  });
};

// API to get list of rooms
export const getRooms = () => {
  return protectedFetch('/rooms');
};

// API to get messages for a specific room
export const getRoomMessages = (roomId) => {
  return protectedFetch(`/rooms/${roomId}/messages`);
};

// API to get all available public rooms
export const getAllRooms = () => {
  return protectedFetch('/rooms/explore');
}

// API to join a specific room
export const joinRoom = (roomId) => {
  return protectedFetch(`/rooms/${roomId}/join`, {
    method: 'POST',
  });
};