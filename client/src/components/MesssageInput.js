import React, { useState } from 'react'


export default function MessageInput({ onSend }) {
const [text, setText] = useState('')


function submit(e) {
e?.preventDefault()
const trimmed = text.trim()
if (!trimmed) return
onSend(trimmed)
setText('')
}


return (
<form className="message-input" onSubmit={submit}>
<input
value={text}
onChange={e => setText(e.target.value)}
placeholder="Write a message..."
aria-label="Message"
/>
<button type="submit" className="send">Send</button>
</form>
)
}