import { html, useState, useEffect, api, navigate, formatDate, formatCost, formatTokens } from '../app.js';
import { PageHeader } from '../components/layout.js';

const PAGE_SIZE = 20;

export function Sessions() {
    const [sessions, setSessions] = useState([]);
    const [agents, setAgents] = useState([]);
    const [agentFilter, setAgentFilter] = useState('');
    const [offset, setOffset] = useState(0);
    const [loading, setLoading] = useState(true);

    // Fetch agent list for the filter dropdown
    useEffect(() => {
        api('/agents').then(res => setAgents(res.data || [])).catch(() => {});
    }, []);

    useEffect(() => {
        setLoading(true);
        let qs = '?limit=' + PAGE_SIZE + '&offset=' + offset;
        if (agentFilter) qs += '&agent=' + encodeURIComponent(agentFilter);
        api('/sessions' + qs)
            .then(res => { setSessions(res.data || []); setLoading(false); })
            .catch(() => { setSessions([]); setLoading(false); });
    }, [agentFilter, offset]);

    const handleAgentChange = (e) => {
        setAgentFilter(e.target.value);
        setOffset(0);
    };

    return html`
        <${PageHeader} title="Sessions" subtitle="Agent chat session history" />

        <div style="display: flex; align-items: center; gap: 1rem; margin-bottom: 1rem;">
            <div class="form-group" style="margin-bottom: 0; min-width: 200px;">
                <select value=${agentFilter} onChange=${handleAgentChange}>
                    <option value="">All agents</option>
                    ${agents.map(a => html`<option key=${a.name} value=${a.name}>${a.name}</option>`)}
                </select>
            </div>
        </div>

        ${loading ? html`<div class="loading">Loading...</div>` : html`
            ${sessions.length === 0 ? html`
                <div class="empty-state">No sessions found</div>
            ` : html`
                <table class="data-table">
                    <thead>
                        <tr>
                            <th>Agent</th>
                            <th>Provider</th>
                            <th>Model</th>
                            <th>Turns</th>
                            <th>Tokens</th>
                            <th>Cost</th>
                            <th>Date</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${sessions.map(s => html`
                            <tr key=${s.id} class="clickable" onClick=${() => navigate('/sessions/' + s.id)}>
                                <td>${s.agent_name}</td>
                                <td>${s.provider}</td>
                                <td>${s.model}</td>
                                <td>${s.total_turns}</td>
                                <td>${formatTokens(s.total_tokens_in + s.total_tokens_out)}</td>
                                <td>${formatCost(s.total_cost_usd)}</td>
                                <td>${formatDate(s.started_at)}</td>
                            </tr>
                        `)}
                    </tbody>
                </table>
            `}

            <div class="pagination">
                <button disabled=${offset === 0} onClick=${() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
                    Prev
                </button>
                <span class="page-info">
                    Showing ${offset + 1}${sessions.length > 0 ? ' - ' + (offset + sessions.length) : ''}
                </span>
                <button disabled=${sessions.length < PAGE_SIZE} onClick=${() => setOffset(offset + PAGE_SIZE)}>
                    Next
                </button>
            </div>
        `}
    `;
}
