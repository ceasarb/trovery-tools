import { html, useState, useEffect, api, navigate, formatDate, statusBadgeClass, formatCost } from '../app.js';
import { PageHeader } from '../components/layout.js';

export function Overview() {
    const [servers, setServers] = useState(null);
    const [agents, setAgents] = useState(null);
    const [evals, setEvals] = useState(null);
    const [sessions, setSessions] = useState(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        Promise.all([
            api('/servers').catch(() => ({ data: [], total: 0 })),
            api('/agents').catch(() => ({ data: [], total: 0 })),
            api('/evals?limit=5').catch(() => ({ data: [], total: 0 })),
            api('/sessions?limit=5').catch(() => ({ data: [], total: 0 })),
        ]).then(([s, a, e, ss]) => {
            setServers(s);
            setAgents(a);
            setEvals(e);
            setSessions(ss);
            setLoading(false);
        });
    }, []);

    if (loading) return html`<div class="loading">Loading...</div>`;

    return html`
        <${PageHeader} title="Overview" subtitle="Workspace summary" />

        <div class="card-grid">
            <div class="stat-card">
                <div class="stat-label">Servers</div>
                <div class="stat-value">${servers?.total ?? 0}</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Agents</div>
                <div class="stat-value">${agents?.total ?? 0}</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Eval Runs</div>
                <div class="stat-value">${evals?.total ?? 0}</div>
            </div>
            <div class="stat-card">
                <div class="stat-label">Sessions</div>
                <div class="stat-value">${sessions?.total ?? 0}</div>
            </div>
        </div>

        <div class="section">
            <h2>Recent Evals</h2>
            ${evals?.data?.length > 0 ? html`
                <table class="data-table">
                    <thead>
                        <tr>
                            <th>Suite</th>
                            <th>Target</th>
                            <th>Source</th>
                            <th>Status</th>
                            <th>Passed</th>
                            <th>Failed</th>
                            <th>Date</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${evals.data.map(e => html`
                            <tr key=${e.id} class="clickable" onClick=${() => navigate('/evals')}>
                                <td>${e.suite_name}</td>
                                <td>${e.target_name}</td>
                                <td><span class="badge badge-purple">${e.source}</span></td>
                                <td><span class="badge ${statusBadgeClass(e.status)}">${e.status}</span></td>
                                <td>${e.passed}</td>
                                <td>${e.failed}</td>
                                <td>${formatDate(e.started_at)}</td>
                            </tr>
                        `)}
                    </tbody>
                </table>
            ` : html`<div class="empty-state">No eval runs yet</div>`}
        </div>

        <div class="section">
            <h2>Recent Sessions</h2>
            ${sessions?.data?.length > 0 ? html`
                <table class="data-table">
                    <thead>
                        <tr>
                            <th>Agent</th>
                            <th>Model</th>
                            <th>Turns</th>
                            <th>Cost</th>
                            <th>Date</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${sessions.data.map(s => html`
                            <tr key=${s.id} class="clickable" onClick=${() => navigate('/sessions/' + s.id)}>
                                <td>${s.agent_name}</td>
                                <td>${s.model}</td>
                                <td>${s.total_turns}</td>
                                <td>${formatCost(s.total_cost_usd)}</td>
                                <td>${formatDate(s.started_at)}</td>
                            </tr>
                        `)}
                    </tbody>
                </table>
            ` : html`<div class="empty-state">No sessions yet</div>`}
        </div>
    `;
}
