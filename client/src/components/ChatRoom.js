import React, { useRef, useEffect } from 'react'
import RoomHeader from './RoomHeader'
import MessageInput from './MesssageInput'


export default function ChatRoom({ room, messages, onSend, onToggleInfo, onOpenRooms }) {
const listRef = useRef(null)


useEffect(() => {
// scroll to bottom on messages change
if (listRef.current) {
listRef.current.scrollTop = listRef.current.scrollHeight
}
}, [messages])


return (
<div className="chat-room">
<RoomHeader room={room} onInfo={onToggleInfo} onOpenRooms={onOpenRooms} />


<div className="messages" ref={listRef}>
{messages.map(m => (
<div key={m.id} className={`message ${m.sender === 'you' ? 'me' : ''}`}>
<div className="msg-avatar">{m.sender[0]?.toUpperCase()}</div>
<div className="msg-body">
<div className="msg-meta">
<strong className="msg-sender">{m.sender}</strong>
<span className="msg-time">{new Date(m.timestamp).toLocaleTimeString()}</span>
</div>
<div className="msg-text">{m.content}</div>
</div>
</div>
))}
</div>


<MessageInput onSend={onSend} />
</div>
)
}