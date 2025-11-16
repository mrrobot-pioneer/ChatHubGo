import { useState } from "react";
import { Eye, EyeOff, MessageCircle, AlertCircle } from "lucide-react";
import { useNavigate, Link } from "react-router-dom";
import { login } from "../utils/api";
import { setToken,setUser } from "../utils/auth";

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
      
      // On successful login, save token and user data
      setToken(data.token);
      setUser(data);
      
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
          <div className="input-grp flex-cl password-field">
            <label htmlFor="password">Password</label>
            <div className="password-wrapper">
              <input
                name="password"
                type={showPass ? "text" : "password"}
                onChange={handleChange}
                required
                autoComplete="password"
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

        <button type="submit" className="button button--primary">Login</button>

        <p style={{color: "var(--text-faded)"}}>
          Don't have an account? <Link to="/register">Register</Link>
        </p>
      </form>
    </div>
  );
}