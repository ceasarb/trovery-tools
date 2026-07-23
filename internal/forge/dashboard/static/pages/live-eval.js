import { html, useState, useEffect, useRef, statusBadgeClass } from '../app.js';
import { PageHeader } from '../components/layout.js';

export function LiveEval() {
    const [events, setEvents] = useState([]);
    const [connected, setConnected] = useState(false);
    const [scenarios, setScenarios] = useState({});
    const [summary, setSummary] = useState(null);
    const wsRef = useRef(null);

    useEffect(() => {
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = new WebSocket(`${protocol}//${location.host}/ws/eval`);
        wsRef.current = ws;

        ws.onopen = () => setConnected(true);
        ws.onclose = () => setConnected(false);
        ws.onerror = () => setConnected(false);

        ws.onmessage = (e) => {
            try {
                const event = JSON.parse(e.data);
                const { type, payload } = event;

                setEvents(prev => [...prev.slice(-500), { ...event, ts: new Date().toISOString() }]);

                if (type === 'eval:scenario') {
                    setScenarios(prev => ({
                        ...prev,
                        [payload.name]: { ...payload, assertions: prev[payload.name]?.assertions || [] }
                    }));
                } else if (type === 'eval:assertion') {
                    setScenarios(prev => {
                        const scenario = prev[payload.scenario] || { name: payload.scenario, assertions: [] };
                        return {
                            ...prev,
                            [payload.scenario]: {
                                ...scenario,
                                assertions: [...scenario.assertions, payload]
                            }
                        };
                    });
                } else if (type === 'eval:done') {
                    setSummary(payload);
                }
            } catch (err) {
                // ignore
            }
        };

        return () => ws.close();
    }, []);

    const reset = () => {
        setEvents([]);
        setScenarios({});
        setSummary(null);
    };

    const scenarioList = Object.values(scenarios);
    const totalAssertions = scenarioList.reduce((sum, s) => sum + (s.assertions?.length || 0), 0);
    const passedAssertions = scenarioList.reduce((sum, s) =>
        sum + (s.assertions?.filter(a => a.passed).length || 0), 0);

    return html`
        <${PageHeader} title="Live Eval Monitor" />

        <div style="display: flex; gap: 16px; margin-bottom: 16px; align-items: center;">
            <span class="badge ${connected ? 'badge-green' : 'badge-red'}">
                ${connected ? 'Connected' : 'Disconnected'}
            </span>
            ${scenarioList.length > 0 && html`
                <span style="color: var(--text-muted); font-size: 0.85em;">
                    ${scenarioList.length} scenarios | ${passedAssertions}/${totalAssertions} assertions passed
                </span>
            `}
            <button class="btn" style="margin-left: auto;" onClick=${reset}>Reset</button>
        </div>

        ${summary && html`
            <div class="card" style="margin-bottom: 16px; border-left: 3px solid ${summary.passed ? 'var(--green)' : 'var(--red)'};">
                <h3 style="margin: 0 0 8px 0;">
                    <span class="badge ${summary.passed ? 'badge-green' : 'badge-red'}">
                        ${summary.passed ? 'PASSED' : 'FAILED'}
                    </span>
                    Eval Complete
                </h3>
                <span style="color: var(--text-muted);">
                    ${summary.total_scenarios} scenarios, ${summary.total_assertions} assertions
                    ${summary.duration ? ' in ' + summary.duration : ''}
                </span>
            </div>
        `}

        ${scenarioList.length === 0
            ? html`
                <div class="card">
                    <div class="empty-state">
                        <p>Waiting for eval events...</p>
                        <p style="font-size: 0.85em; color: var(--text-muted);">
                            Run an eval to see real-time progress: ajnt agent eval [name]
                        </p>
                    </div>
                </div>`
            : scenarioList.map(scenario => html`
                <div class="card" key=${scenario.name} style="margin-bottom: 12px;">
                    <h3 style="margin: 0 0 8px 0;">
                        <span class="badge ${
                            scenario.status === 'running' ? 'badge-blue' :
                            scenario.assertions?.every(a => a.passed) ? 'badge-green' : 'badge-red'
                        }">
                            ${scenario.status || 'running'}
                        </span>
                        ${' '}${scenario.name}
                    </h3>
                    ${scenario.assertions?.length > 0 && html`
                        <div style="display: flex; flex-direction: column; gap: 4px; margin-top: 8px;">
                            ${scenario.assertions.map((a, i) => html`
                                <div key=${i} style="display: flex; gap: 8px; align-items: center; font-size: 0.9em;">
                                    <span class="badge ${a.passed ? 'badge-green' : 'badge-red'}" style="min-width: 50px; text-align: center;">
                                        ${a.passed ? 'PASS' : 'FAIL'}
                                    </span>
                                    <span style="color: var(--text-secondary);">${a.type}: ${a.description || ''}</span>
                                    ${a.message && !a.passed ? html`
                                        <span style="color: var(--red); font-size: 0.85em;">${a.message}</span>
                                    ` : ''}
                                </div>
                            `)}
                        </div>
                    `}
                </div>
            `)
        }
    `;
}
