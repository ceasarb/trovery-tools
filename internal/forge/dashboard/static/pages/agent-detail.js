import { html, useState, useEffect, api } from '../app.js';
import { PageHeader } from '../components/layout.js';
import { DataTable } from '../components/data-table.js';

export function AgentDetail({ name }) {
    const [agent, setAgent] = useState(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);

    useEffect(() => {
        setLoading(true);
        api('/agents/' + encodeURIComponent(name))
            .then(res => { setAgent(res.data); setLoading(false); })
            .catch(err => { setError(err.message); setLoading(false); });
    }, [name]);

    if (loading) return html`<div class="loading">Loading...</div>`;
    if (error) return html`<div class="empty-state"><p>Error: ${error}</p></div>`;
    if (!agent) return html`<div class="empty-state"><p>Agent not found</p></div>`;

    const serverColumns = [
        { key: 'name', label: 'Name' },
        { key: 'path', label: 'Path' },
        { key: 'command', label: 'Command' },
    ];

    return html`
        <${PageHeader} title=${agent.name} subtitle="Agent configuration" />

        <div class="section">
            <h2>Configuration</h2>
            <div class="detail-grid">
                <div class="detail-item">
                    <div class="label">Provider</div>
                    <div class="value">${agent.provider}</div>
                </div>
                <div class="detail-item">
                    <div class="label">Model</div>
                    <div class="value">${agent.model}</div>
                </div>
                <div class="detail-item">
                    <div class="label">Max Tool Calls</div>
                    <div class="value">${agent.settings?.max_tool_calls || 'default'}</div>
                </div>
                <div class="detail-item">
                    <div class="label">Timeout</div>
                    <div class="value">${agent.settings?.timeout_secs ? agent.settings.timeout_secs + 's' : 'default'}</div>
                </div>
                <div class="detail-item">
                    <div class="label">Namespacing</div>
                    <div class="value">${agent.settings?.namespacing || 'auto'}</div>
                </div>
            </div>
        </div>

        <div class="section">
            <h2>Servers (${agent.servers?.length || 0})</h2>
            <${DataTable}
                columns=${serverColumns}
                data=${agent.servers}
                emptyMessage="No servers wired to this agent"
            />
        </div>

        ${agent.system_prompt ? html`
            <div class="section">
                <h2>System Prompt</h2>
                <pre class="code-block">${agent.system_prompt}</pre>
            </div>
        ` : null}
    `;
}
