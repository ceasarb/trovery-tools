import { html, useState, useEffect, useRef, api, formatCost, formatTokens } from '../app.js';
import { PageHeader } from '../components/layout.js';

export function ChatPlayground() {
    const [agents, setAgents] = useState([]);
    const [selectedAgent, setSelectedAgent] = useState('');
    const [messages, setMessages] = useState([]);
    const [input, setInput] = useState('');
    const [sending, setSending] = useState(false);
    const [connected, setConnected] = useState(false);
    const [lastUsage, setLastUsage] = useState(null);
    const wsRef = useRef(null);
    const messagesEndRef = useRef(null);
    const currentAssistantRef = useRef('');

    // Load agent list
    useEffect(() => {
        api('/agents').then(data => {
            const list = data.data || [];
            setAgents(list);
            if (list.length > 0) setSelectedAgent(list[0].name);
        }).catch(() => {});
    }, []);

    // Auto-scroll
    useEffect(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, [messages]);

    const connectWS = () => {
        if (wsRef.current) wsRef.current.close();

        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = new WebSocket(`${protocol}//${location.host}/ws/chat`);
        wsRef.current = ws;

        ws.onopen = () => setConnected(true);
        ws.onclose = () => setConnected(false);
        ws.onerror = () => setConnected(false);

        ws.onmessage = (e) => {
            try {
                const event = JSON.parse(e.data);
                const { type, payload } = event;

                if (type === 'agent:text') {
                    currentAssistantRef.current += payload.text;
                    setMessages(prev => {
                        const updated = [...prev];
                        const last = updated[updated.length - 1];
                        if (last && last.role === 'assistant' && !last.done) {
                            updated[updated.length - 1] = { ...last, content: currentAssistantRef.current };
                        }
                        return updated;
                    });
                } else if (type === 'agent:tool_start') {
                    setMessages(prev => [...prev, { role: 'tool', tool: payload.tool, status: 'running' }]);
                } else if (type === 'agent:tool_end') {
                    setMessages(prev => {
                        const updated = [...prev];
                        const idx = updated.findLastIndex(m => m.role === 'tool' && m.tool === payload.tool && m.status === 'running');
                        if (idx >= 0) {
                            updated[idx] = { ...updated[idx], status: 'done', summary: payload.summary };
                        }
                        return updated;
                    });
                } else if (type === 'agent:done') {
                    setSending(false);
                    setLastUsage(payload);
                    setMessages(prev => {
                        const updated = [...prev];
                        const last = updated[updated.length - 1];
                        if (last && last.role === 'assistant') {
                            updated[updated.length - 1] = { ...last, done: true };
                        }
                        return updated;
                    });
                } else if (type === 'error') {
                    setSending(false);
                    setMessages(prev => [...prev, { role: 'error', content: payload.error }]);
                }
            } catch (err) {
                // ignore
            }
        };
    };

    const sendMessage = () => {
        if (!input.trim() || !selectedAgent || sending) return;

        if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
            connectWS();
            // Wait for connection then send
            setTimeout(() => sendMessageInner(), 500);
        } else {
            sendMessageInner();
        }
    };

    const sendMessageInner = () => {
        const msg = input.trim();
        setInput('');
        setSending(true);
        currentAssistantRef.current = '';

        setMessages(prev => [
            ...prev,
            { role: 'user', content: msg },
            { role: 'assistant', content: '', done: false }
        ]);

        wsRef.current.send(JSON.stringify({ agent: selectedAgent, message: msg }));
    };

    const handleKeyDown = (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            sendMessage();
        }
    };

    const clearChat = () => {
        setMessages([]);
        setLastUsage(null);
        currentAssistantRef.current = '';
    };

    return html`
        <${PageHeader} title="Chat Playground" />

        <div style="display: flex; gap: 16px; margin-bottom: 16px; align-items: center;">
            <select value=${selectedAgent} onChange=${(e) => setSelectedAgent(e.target.value)}
                    style="min-width: 200px;">
                ${agents.map(a => html`<option key=${a.name} value=${a.name}>${a.name}</option>`)}
            </select>
            <span class="badge ${connected ? 'badge-green' : 'badge-gray'}">
                ${connected ? 'Connected' : 'Not connected'}
            </span>
            ${lastUsage && html`
                <span style="color: var(--text-muted); font-size: 0.85em; margin-left: auto;">
                    ${formatTokens(lastUsage.tokens_in + lastUsage.tokens_out)} tokens
                    | ${formatCost(lastUsage.cost_usd)}
                    | ${lastUsage.tool_calls} tool calls
                </span>
            `}
            <button class="btn" onClick=${clearChat}>Clear</button>
        </div>

        <div class="card" style="height: 500px; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 8px;">
            ${messages.length === 0
                ? html`<div class="empty-state"><p>Send a message to start chatting with ${selectedAgent || 'an agent'}.</p></div>`
                : messages.map((msg, i) => html`
                    <div key=${i} class="turn-bubble ${msg.role}">
                        ${msg.role === 'tool'
                            ? html`
                                <div class="tool-call-card">
                                    <span class="badge ${msg.status === 'running' ? 'badge-blue' : 'badge-green'}">
                                        ${msg.tool}
                                    </span>
                                    ${msg.summary ? html`<span style="color: var(--text-muted); font-size: 0.85em;"> ${msg.summary}</span>` : ''}
                                </div>`
                            : msg.role === 'error'
                            ? html`<div style="color: var(--red);">${msg.content}</div>`
                            : html`<div>${msg.content}${!msg.done && msg.role === 'assistant' ? html`<span class="cursor-blink">|</span>` : ''}</div>`
                        }
                    </div>
                `)
            }
            <div ref=${messagesEndRef} />
        </div>

        <div style="display: flex; gap: 8px; margin-top: 12px;">
            <textarea
                value=${input}
                onInput=${(e) => setInput(e.target.value)}
                onKeyDown=${handleKeyDown}
                placeholder="Type a message... (Enter to send)"
                rows="2"
                style="flex: 1; resize: none;"
                disabled=${sending}
            />
            <button class="btn" onClick=${sendMessage} disabled=${sending || !input.trim()}>
                ${sending ? 'Sending...' : 'Send'}
            </button>
        </div>
    `;
}
