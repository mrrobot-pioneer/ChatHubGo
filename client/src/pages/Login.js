import { useState } from "react";
import { Eye, EyeOff, MessageCircle } from "lucide-react";
import { useNavigate, Link } from "react-router-dom";
import { login } from "../utils/api";
import { setToken } from "../utils/auth";

import "../styles/Auth.css";

export default function Login() {
  const [form, setForm] = useState({ username: "", password: "" });
  const [error, setError] = useState("");
  const [showPass, setShowPass] = useState(false);
  const navigate = useNavigate();

  const handleChange = (e) =>
    setForm({ ...form, [e.target.name]: e.target.value });

  const handleSubmit = async (e) => {
    e.preventDefault();
    try {
      const data = await login(form);
      setToken(data.token);
      navigate("/chat");
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <div className="auth-page flex-cl">
      <form onSubmit={handleSubmit} className="flex-cl">
        <div className="flex">
          <MessageCircle size={26} color="var(--primary-color)"/>
          <h2>ChatHub</h2>
        </div>

        <p style={{color: "var(--text-faded)"}}>
          Real-time messaging platform
        </p>

        <div className="inputs flex-cl">
          <div className="input-grp flex-cl">
            <label htmlFor="username">Username</label>
            <input name="username" onChange={handleChange} required />
          </div>
          <div className="input-grp flex-cl password-field">
            <label htmlFor="password">Password</label>
            <div className="password-wrapper">
              <input
                name="password"
                type={showPass ? "text" : "password"}
                onChange={handleChange}
                required
              />
              <div
                className="toggle-icon"
                onClick={() => setShowPass(!showPass)}
                style={{ cursor: "pointer" }}
              >
                {showPass ? <EyeOff size={18} /> : <Eye size={18} />}
              </div>
            </div>
          </div>
        </div>

        {error && <p className="small-text" style={{ color: "red", width: "100%" }}>{error}</p>}

        <button type="submit" className="primary-btn">Login</button>

        <p style={{color: "var(--text-faded)"}}>
          Don't have an account? <Link to="/register">Register</Link>
        </p>
      </form>
    </div>
  );
}