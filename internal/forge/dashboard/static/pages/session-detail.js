import { html, useState, useEffect, api, formatDate, formatCost, formatTokens, formatDuration } from '../app.js';
import { PageHeader } from '../components/layout.js';

export function SessionDetail({ id }) {
    const [data, setData] = useState(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);

    useEffect(() => {
        setLoading(true);
        api('/sessions/' + id)
            .then(res => { setData(res.data); setLoading(false); })
            .catch(err => { setError(err.message); setLoading(false); });
    }, [id]);

    if (loading) return html`<div class="loading">Loading...</div>`;
    if (error) return html`<div class="empty-state"><p>Error: ${error}</p></div>`;
    if (!data) return html`<div class="empty-state"><p>Session not found</p></div>`;

    const session = data.session;
    const turns = data.turns || [];

    const handleExport = () => {
        window.open('/api/sessions/' + id + '/export', '_blank');
    };

    return html`
        <${PageHeader} title=${'Session: ' + session.agent_name} subtitle=${'ID: ' + session.id} />

        <div class="session-header card" style="margin-bottom: 1.5rem;">
            <div style="display: flex; gap: 2rem; flex-wrap: wrap; align-items: flex-start;">
                <div class="meta-item">
                    <span class="label">Provider</span>
                    <span class="value">${session.provider}</span>
                </div>
                <div class="meta-item">
                    <span class="label">Model</span>
                    <span class="value">${session.model}</span>
                </div>
                <div class="meta-item">
                    <span class="label">Turns</span>
                    <span class="value">${session.total_turns}</span>
                </div>
                <div class="meta-item">
                    <span class="label">Tokens</span>
                    <span class="value">${formatTokens(session.total_tokens_in + session.total_tokens_out)}</span>
                </div>
                <div class="meta-item">
                    <span class="label">Cost</span>
                    <span class="value">${formatCost(session.total_cost_usd)}</span>
                </div>
                <div class="meta-item">
                    <span class="label">Started</span>
                    <span class="value">${formatDate(session.started_at)}</span>
                </div>
                <div style="margin-left: auto;">
                    <button class="btn btn-secondary" onClick=${handleExport}>Export JSON</button>
                </div>
            </div>
            ${session.summary ? html`
                <div style="margin-top: 1rem; padding-top: 1rem; border-top: 1px solid var(--border);">
                    <div style="font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); margin-bottom: 0.25rem;">Summary</div>
                    <div style="color: var(--text-secondary); font-size: 0.9rem;">${session.summary}</div>
                </div>
            ` : null}
        </div>

        <div class="section">
            <h2>Conversation</h2>
            <div class="turn-list">
                ${turns.map(turn => html`
                    <${Turn} key=${turn.id} turn=${turn} />
                `)}
                ${turns.length === 0 ? html`<div class="empty-state">No turns recorded</div>` : null}
            </div>
        </div>
    `;
}

function Turn({ turn }) {
    return html`
        <div>
            <div class="turn-bubble ${turn.role}">
                <div class="turn-role">${turn.role}</div>
                ${turn.content}
            </div>
            ${turn.tool_calls?.length > 0 ? turn.tool_calls.map(tc => html`
                <${ToolCallCard} key=${tc.id} call=${tc} />
            `) : null}
        </div>
    `;
}

function ToolCallCard({ call }) {
    const [expanded, setExpanded] = useState(false);

    return html`
        <div class="tool-call-card">
            <div class="tool-call-header" onClick=${() => setExpanded(!expanded)}>
                <span class="expand-icon ${expanded ? 'open' : ''}">\u25B6</span>
                <span class="tool-name">${call.tool_name}</span>
                <span class="server-name">${call.server_name}</span>
                ${call.duration_ms != null ? html`
                    <span class="duration">${formatDuration(call.duration_ms)}</span>
                ` : null}
                ${call.error ? html`<span class="badge badge-red">error</span>` : null}
            </div>
            ${expanded ? html`
                <div class="tool-call-body">
                    ${call.arguments ? html`
                        <div class="tool-call-section">
                            <div class="section-label">Arguments</div>
                            <pre>${formatJSON(call.arguments)}</pre>
                        </div>
                    ` : null}
                    ${call.result ? html`
                        <div class="tool-call-section">
                            <div class="section-label">Result</div>
                            <pre>${formatJSON(call.result)}</pre>
                        </div>
                    ` : null}
                    ${call.error ? html`
                        <div class="tool-call-section">
                            <div class="section-label">Error</div>
                            <div class="tool-call-error">${call.error}</div>
                        </div>
                    ` : null}
                </div>
            ` : null}
        </div>
    `;
}

function formatJSON(str) {
    if (!str) return '';
    try {
        return JSON.stringify(JSON.parse(str), null, 2);
    } catch {
        return str;
    }
}
