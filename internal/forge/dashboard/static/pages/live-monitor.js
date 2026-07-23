import { html, useState, useEffect, useRef, formatCost } from '../app.js';
import { PageHeader } from '../components/layout.js';

export function LiveMonitor() {
    const [events, setEvents] = useState([]);
    const [connected, setConnected] = useState(false);
    const [activeSessions, setActiveSessions] = useState(0);
    const wsRef = useRef(null);
    const eventsEndRef = useRef(null);

    useEffect(() => {
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = new WebSocket(`${protocol}//${location.host}/ws/events`);
        wsRef.current = ws;

        ws.onopen = () => setConnected(true);
        ws.onclose = () => {
            setConnected(false);
            // Reconnect after 3s
            setTimeout(() => {
                if (wsRef.current === ws) {
                    wsRef.current = null;
                    // Trigger re-mount by updating state
                }
            }, 3000);
        };
        ws.onerror = () => setConnected(false);

        ws.onmessage = (e) => {
            try {
                const event = JSON.parse(e.data);
                setEvents(prev => [...prev.slice(-200), { ...event, ts: new Date().toISOString() }]);

                if (event.type === 'session:start') {
                    setActiveSessions(n => n + 1);
                } else if (event.type === 'session:end') {
                    setActiveSessions(n => Math.max(0, n - 1));
                }
            } catch (err) {
                // ignore parse errors
            }
        };

        return () => ws.close();
    }, []);

    useEffect(() => {
        eventsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, [events]);

    const clearEvents = () => setEvents([]);

    const eventColor = (type) => {
        if (type?.includes('text')) return 'var(--text-primary)';
        if (type?.includes('tool_start')) return 'var(--blue)';
        if (type?.includes('tool_end')) return 'var(--green)';
        if (type?.includes('done')) return 'var(--green)';
        if (type?.includes('error')) return 'var(--red)';
        if (type?.includes('eval')) return 'var(--yellow)';
        if (type?.includes('session')) return 'var(--accent)';
        return 'var(--text-muted)';
    };

    return html`
        <${PageHeader} title="Live Monitor" />

        <div class="monitor-controls" style="display: flex; gap: 16px; margin-bottom: 16px; align-items: center;">
            <span class="badge ${connected ? 'badge-green' : 'badge-red'}">
                ${connected ? 'Connected' : 'Disconnected'}
            </span>
            <span class="badge badge-blue">Active: ${activeSessions}</span>
            <span style="color: var(--text-muted); font-size: 0.85em;">${events.length} events</span>
            <button class="btn" style="margin-left: auto;" onClick=${clearEvents}>Clear</button>
        </div>

        <div class="card" style="height: 600px; overflow-y: auto; font-family: monospace; font-size: 0.85em; padding: 12px;">
            ${events.length === 0
                ? html`<div class="empty-state"><p>Waiting for events...</p><p style="font-size: 0.85em; color: var(--text-muted);">Events appear when agents are invoked via the HTTP server or chat playground.</p></div>`
                : events.map((event, i) => html`
                    <div key=${i} style="margin-bottom: 4px; display: flex; gap: 8px;">
                        <span style="color: var(--text-muted); min-width: 80px; font-size: 0.8em;">
                            ${new Date(event.ts).toLocaleTimeString()}
                        </span>
                        <span style="color: ${eventColor(event.type)}; min-width: 140px;">
                            ${event.type}
                        </span>
                        <span style="color: var(--text-secondary);">
                            ${formatPayload(event.payload)}
                        </span>
                    </div>
                `)
            }
            <div ref=${eventsEndRef} />
        </div>
    `;
}

function formatPayload(payload) {
    if (!payload) return '';
    if (payload.text) return payload.text.substring(0, 120);
    if (payload.tool) return payload.tool + (payload.summary ? ': ' + payload.summary.substring(0, 80) : '');
    if (payload.agent) return payload.agent + (payload.cost_usd ? ' (' + formatCost(payload.cost_usd) + ')' : '');
    if (payload.error) return payload.error;
    return JSON.stringify(payload).substring(0, 120);
}
