import { useState } from "react";
import { Eye, EyeOff, MessageCircle, AlertCircle } from "lucide-react";
import { useNavigate, Link } from "react-router-dom";
import { register } from "../utils/api";
import { setToken, setUser } from "../utils/auth";

import "../styles/Auth.css";

export default function Register() {
  const [form, setForm] = useState({
    username: "",
    email: "",
    password1: "",
    password2: "",
  });
  const [error, setError] = useState("");
  const [showPass1, setShowPass1] = useState(false);
  const [showPass2, setShowPass2] = useState(false);
  const [strength, setStrength] = useState("");
  const navigate = useNavigate();

  const handleChange = (e) => {
    const { name, value } = e.target;
    setForm({ ...form, [name]: value });

    // Password strength validation (real-time)
    if (name === "password1") {
      evaluateStrength(value);
    }
  };

  // Function to evaluate password strength
  const evaluateStrength = (password) => {
    if (password.length < 6) setStrength("Weak");
    else if (password.match(/[A-Z]/) && password.match(/\d/))
      setStrength("Strong");
    else setStrength("Medium");
  };

  const handleSubmit = async (e) => {
    e.preventDefault();

    // Confirm passwords match
    if (form.password1 !== form.password2) {
      setError("Passwords do not match");
      return;
    }

    // ensure password strength before submission
    if (strength === "Weak") {
      setError("Please use a stronger password");
      return;
    }

    try {
      const data = await register({
        username: form.username,
        email: form.email,
        password: form.password1,
      });

      // On successful registration, save token and user data
      setToken(data.token);
      setUser(data);
      navigate("/chat");
    } catch (err) {
      setError(err.message || "Registration failed");
    }
  };

  return (
    <div className="auth-page flex-cl">
      <form onSubmit={handleSubmit} className="flex-cl">
        <div className="flex">
          <MessageCircle size={26} color="var(--primary-color)"/>
          <h2>ChatHub</h2>
        </div>

        <p style={{ color: "var(--text-faded)" }}>
          Real-time messaging platform
        </p>

        {error && 
          <div className="auth--error">
            <AlertCircle size={16}/>
            <p>{error}</p>
          </div>
        }

        <div className="inputs flex-cl">
          <div className="input-grp flex-cl">
            <label htmlFor="username">Username</label>
            <input name="username" onChange={handleChange} required autoComplete="username"/>
          </div>

          <div className="input-grp flex-cl">
            <label htmlFor="email">Email</label>
            <input
              name="email"
              type="email"
              onChange={handleChange}
              required
              autoComplete="email"
            />
          </div>

          <div className="input-grp flex-cl password-field">
            <label htmlFor="password1">Password</label>
            <div className="password-wrapper">
              <input
                name="password1"
                type={showPass1 ? "text" : "password"}
                onChange={handleChange}
                required
                autoComplete="password"
              />
              <div
                className="toggle-icon"
                onClick={() => setShowPass1(!showPass1)}
                style={{ cursor: "pointer" }}
              >
                {showPass1 ? <EyeOff size={18} /> : <Eye size={18} />}
              </div>
            </div>
            {strength && (
              <p
                style={{
                  color:
                    strength === "Strong"
                      ? "green"
                      : strength === "Medium"
                      ? "orange"
                      : "red",
                }}
                className="small-text"
              >
                Strength: {strength}
              </p>
            )}
          </div>

          <div className="input-grp flex-cl password-field">
            <label htmlFor="password2">Confirm Password</label>
            <div className="password-wrapper">
              <input
                name="password2"
                type={showPass2 ? "text" : "password"}
                onChange={handleChange}
                required
                autoComplete="confirm password"
              />
              <div
                className="toggle-icon"
                onClick={() => setShowPass2(!showPass2)}
                style={{ cursor: "pointer" }}
              >
                {showPass2 ? <EyeOff size={18} /> : <Eye size={18} />}
              </div>
            </div>
          </div>
        </div>

        <button type="submit" className="button button--primary">
          Register
        </button>

        <p style={{ color: "var(--text-faded)" }}>
          Already have an account? <Link to="/login">Login</Link>
        </p>
      </form>
    </div>
  );
}
