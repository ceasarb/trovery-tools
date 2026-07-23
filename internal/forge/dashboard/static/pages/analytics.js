import { html, useState, useEffect, useRef, api } from '../app.js';
import { PageHeader } from '../components/layout.js';
import { renderLineChart, renderBarChart } from '../components/charts.js';

const DAY_OPTIONS = [7, 14, 30, 90];

export function Analytics() {
    const [days, setDays] = useState(30);
    const [cost, setCost] = useState(null);
    const [tokens, setTokens] = useState(null);
    const [usage, setUsage] = useState(null);
    const [errors, setErrors] = useState(null);
    const [latency, setLatency] = useState(null);
    const rendered = useRef(false);

    useEffect(() => {
        rendered.current = false;
        const qs = '?days=' + days;
        Promise.all([
            api('/analytics/cost' + qs).catch(() => ({ data: [] })),
            api('/analytics/tokens' + qs).catch(() => ({ data: [] })),
            api('/analytics/usage' + qs).catch(() => ({ data: [] })),
            api('/analytics/errors' + qs).catch(() => ({ data: [] })),
            api('/analytics/latency' + qs).catch(() => ({ data: [] })),
        ]).then(([c, t, u, e, l]) => {
            setCost(c.data);
            setTokens(t.data);
            setUsage(u.data);
            setErrors(e.data);
            setLatency(l.data);
        });
    }, [days]);

    // Render charts after data is set and DOM is ready
    useEffect(() => {
        if (rendered.current) return;
        if (!cost || !tokens || !usage || !errors || !latency) return;
        rendered.current = true;

        // Cost over time
        if (cost.length > 0) {
            renderLineChart('chart-cost',
                cost.map(p => p.date),
                [{ label: 'Cost (USD)', data: cost.map(p => p.value), color: '#7c3aed' }],
            );
        }

        // Tokens over time
        if (tokens.length > 0) {
            renderBarChart('chart-tokens',
                tokens.map(p => p.date),
                tokens.map(p => p.value),
                null, false,
            );
        }

        // Tool usage
        if (usage.length > 0) {
            renderBarChart('chart-usage',
                usage.map(p => p.tool_name),
                usage.map(p => p.count),
                null, true,
            );
        }

        // Errors over time
        if (errors.length > 0) {
            renderLineChart('chart-errors',
                errors.map(p => p.date),
                [{ label: 'Errors', data: errors.map(p => p.value), color: '#ef4444' }],
            );
        }

        // Latency percentiles
        if (latency.length > 0) {
            renderLineChart('chart-latency',
                latency.map(p => p.date),
                [
                    { label: 'p50', data: latency.map(p => p.p50), color: '#22c55e' },
                    { label: 'p95', data: latency.map(p => p.p95), color: '#eab308' },
                    { label: 'p99', data: latency.map(p => p.p99), color: '#ef4444' },
                ],
            );
        }
    }, [cost, tokens, usage, errors, latency]);

    return html`
        <${PageHeader} title="Analytics" subtitle="Usage and performance metrics" />

        <div class="filter-tabs" style="margin-bottom: 1.5rem;">
            ${DAY_OPTIONS.map(d => html`
                <button key=${d}
                    class="filter-tab ${days === d ? 'active' : ''}"
                    onClick=${() => setDays(d)}>
                    ${d} days
                </button>
            `)}
        </div>

        <div class="chart-grid">
            <div class="chart-panel">
                <h3>Cost Over Time</h3>
                <div style="height: 260px;">
                    ${cost && cost.length === 0 ? html`<div class="empty-state">No cost data</div>`
                        : html`<canvas id="chart-cost"></canvas>`}
                </div>
            </div>

            <div class="chart-panel">
                <h3>Token Usage</h3>
                <div style="height: 260px;">
                    ${tokens && tokens.length === 0 ? html`<div class="empty-state">No token data</div>`
                        : html`<canvas id="chart-tokens"></canvas>`}
                </div>
            </div>

            <div class="chart-panel">
                <h3>Tool Usage Frequency</h3>
                <div style="height: 260px;">
                    ${usage && usage.length === 0 ? html`<div class="empty-state">No usage data</div>`
                        : html`<canvas id="chart-usage"></canvas>`}
                </div>
            </div>

            <div class="chart-panel">
                <h3>Error Rates</h3>
                <div style="height: 260px;">
                    ${errors && errors.length === 0 ? html`<div class="empty-state">No error data</div>`
                        : html`<canvas id="chart-errors"></canvas>`}
                </div>
            </div>

            <div class="chart-panel">
                <h3>Latency Percentiles (ms)</h3>
                <div style="height: 260px;">
                    ${latency && latency.length === 0 ? html`<div class="empty-state">No latency data</div>`
                        : html`<canvas id="chart-latency"></canvas>`}
                </div>
            </div>
        </div>
    `;
}
