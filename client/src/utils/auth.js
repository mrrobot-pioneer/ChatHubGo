export const getToken = () => localStorage.getItem("token");
export const setToken = (token) => localStorage.setItem("token", token);

export const isLoggedIn = () => !!getToken();

const USER_KEY = "user-data";

/**
 * Saves user details (id, username, etc.) to local storage.
 * @param {object} data - The response data from the login API call
 */
export const setUser = (data) => {
    const userData = {
        id: data.user_id,
        username: data.username,
        avatar: data.username ? data.username[0].toUpperCase() : 'U',
    };
    localStorage.setItem(USER_KEY, JSON.stringify(userData));
};

/**
 * Retrieves the logged-in user details from local storage.
 * @returns {object | null} The user object or null if not found.
 */
export const getUser = () => {
    const user = localStorage.getItem(USER_KEY);
    return user ? JSON.parse(user) : null;
};

/**
 * Removes both the token and user data on logout.
 */
export const removeAuth = () => {
    localStorage.removeItem("token");
    localStorage.removeItem(USER_KEY);
};