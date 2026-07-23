import { html, useState, useEffect, api, formatDate, statusBadgeClass } from '../app.js';
import { PageHeader } from '../components/layout.js';

export function Evals() {
    const [runs, setRuns] = useState([]);
    const [source, setSource] = useState('');
    const [loading, setLoading] = useState(true);
    const [expandedId, setExpandedId] = useState(null);
    const [details, setDetails] = useState({});

    useEffect(() => {
        setLoading(true);
        const qs = source ? '?source=' + source : '';
        api('/evals' + qs)
            .then(res => { setRuns(res.data); setLoading(false); })
            .catch(() => { setRuns([]); setLoading(false); });
    }, [source]);

    const toggleExpand = (id) => {
        if (expandedId === id) {
            setExpandedId(null);
            return;
        }
        setExpandedId(id);
        if (!details[id]) {
            api('/evals/' + id).then(res => {
                setDetails(prev => ({ ...prev, [id]: res.data }));
            }).catch(() => {});
        }
    };

    const SOURCES = ['', 'server', 'agent'];

    return html`
        <${PageHeader} title="Evals" subtitle="Evaluation run history" />

        <div class="filter-tabs">
            ${SOURCES.map(s => html`
                <button key=${s}
                    class="filter-tab ${source === s ? 'active' : ''}"
                    onClick=${() => setSource(s)}>
                    ${s === '' ? 'All' : s.charAt(0).toUpperCase() + s.slice(1) + 's'}
                </button>
            `)}
        </div>

        ${loading ? html`<div class="loading">Loading...</div>` : html`
            ${runs.length === 0 ? html`
                <div class="empty-state">No eval runs found</div>
            ` : html`
                <table class="data-table">
                    <thead>
                        <tr>
                            <th></th>
                            <th>Suite</th>
                            <th>Target</th>
                            <th>Source</th>
                            <th>Status</th>
                            <th>Passed</th>
                            <th>Failed</th>
                            <th>Skipped</th>
                            <th>Date</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${runs.map(run => html`
                            <tr key=${run.id} class="clickable" onClick=${() => toggleExpand(run.id)}>
                                <td>
                                    <span class="expand-icon ${expandedId === run.id ? 'open' : ''}">\u25B6</span>
                                </td>
                                <td>${run.suite_name}</td>
                                <td>${run.target_name}</td>
                                <td><span class="badge badge-purple">${run.source}</span></td>
                                <td><span class="badge ${statusBadgeClass(run.status)}">${run.status}</span></td>
                                <td>${run.passed}</td>
                                <td>${run.failed}</td>
                                <td>${run.skipped}</td>
                                <td>${formatDate(run.started_at)}</td>
                            </tr>
                            ${expandedId === run.id ? html`
                                <tr key=${run.id + '-detail'}>
                                    <td colspan="9" style="padding: 0;">
                                        <${EvalResults} detail=${details[run.id]} />
                                    </td>
                                </tr>
                            ` : null}
                        `)}
                    </tbody>
                </table>
            `}
        `}
    `;
}

function EvalResults({ detail }) {
    if (!detail) return html`<div class="loading" style="padding: 1rem;">Loading results...</div>`;

    const results = detail.results || [];
    if (results.length === 0) return html`<div style="padding: 1rem; color: var(--text-muted);">No individual results</div>`;

    return html`
        <div style="padding: 0.75rem 1rem 0.75rem 2.5rem; background: var(--bg-primary);">
            <table class="data-table" style="border: none;">
                <thead>
                    <tr>
                        <th>Scenario</th>
                        <th>Status</th>
                        <th>Duration</th>
                        <th>Error</th>
                    </tr>
                </thead>
                <tbody>
                    ${results.map(r => html`
                        <tr key=${r.id}>
                            <td>${r.scenario_name}</td>
                            <td><span class="badge ${statusBadgeClass(r.status)}">${r.status}</span></td>
                            <td>${r.duration_ms != null ? r.duration_ms + 'ms' : '-'}</td>
                            <td style="color: var(--red); font-size: 0.85rem;">${r.error_message || '-'}</td>
                        </tr>
                    `)}
                </tbody>
            </table>
        </div>
    `;
}
