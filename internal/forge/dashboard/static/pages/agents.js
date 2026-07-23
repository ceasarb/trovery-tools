import { html, useState, useEffect, api, navigate } from '../app.js';
import { PageHeader } from '../components/layout.js';

export function Agents() {
    const [agents, setAgents] = useState(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        api('/agents').then(res => { setAgents(res.data); setLoading(false); })
            .catch(() => { setAgents([]); setLoading(false); });
    }, []);

    if (loading) return html`<div class="loading">Loading...</div>`;

    if (!agents || agents.length === 0) {
        return html`
            <${PageHeader} title="Agents" subtitle="AI agents in this workspace" />
            <div class="empty-state">
                <div class="empty-icon">\u2726</div>
                <p>No agents found. Run ajnt agent create to scaffold one.</p>
            </div>
        `;
    }

    return html`
        <${PageHeader} title="Agents" subtitle="AI agents in this workspace" />
        <div class="card-grid">
            ${agents.map(a => html`
                <div key=${a.name} class="agent-card" onClick=${() => navigate('/agents/' + encodeURIComponent(a.name))}>
                    <h3>${a.name}</h3>
                    <div class="meta">
                        <span>Provider: ${a.provider}</span>
                        <span>Model: ${a.model}</span>
                    </div>
                </div>
            `)}
        </div>
    `;
}
