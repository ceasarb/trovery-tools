import React from 'react';
import { createRoot } from 'react-dom/client';
import htm from 'htm';

// Bind htm to React.createElement for JSX-like syntax without a build step.
export const html = htm.bind(React.createElement);

// Re-export hooks for pages to import.
export const { useState, useEffect, useRef, useCallback, useMemo } = React;

// ---------- API helper ----------

export async function api(path) {
    const res = await fetch('/api' + path);
    if (!res.ok) {
        const body = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(body.error || res.statusText);
    }
    return res.json();
}

export async function apiPost(path, body) {
    const res = await fetch('/api' + path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    });
    if (!res.ok) {
        const data = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(data.error || res.statusText);
    }
    return res.json();
}

// ---------- Router ----------

export function navigate(path) {
    window.location.hash = path;
}

export function useRoute() {
    const [route, setRoute] = useState(window.location.hash.slice(1) || '/');

    useEffect(() => {
        const onChange = () => setRoute(window.location.hash.slice(1) || '/');
        window.addEventListener('hashchange', onChange);
        return () => window.removeEventListener('hashchange', onChange);
    }, []);

    return route;
}

// Parse route parameters: matchRoute('/agents/:name', '/agents/my-agent') => { name: 'my-agent' }
export function matchRoute(pattern, path) {
    const patternParts = pattern.split('/');
    const pathParts = path.split('/');
    if (patternParts.length !== pathParts.length) return null;

    const params = {};
    for (let i = 0; i < patternParts.length; i++) {
        if (patternParts[i].startsWith(':')) {
            params[patternParts[i].slice(1)] = decodeURIComponent(pathParts[i]);
        } else if (patternParts[i] !== pathParts[i]) {
            return null;
        }
    }
    return params;
}

// ---------- Formatting helpers ----------

export function formatDate(dateStr) {
    if (!dateStr) return '-';
    const d = new Date(dateStr);
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
        + ' ' + d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
}

export function formatCost(usd) {
    if (usd == null || usd === 0) return '$0.00';
    if (usd < 0.01) return '$' + usd.toFixed(4);
    return '$' + usd.toFixed(2);
}

export function formatTokens(n) {
    if (n == null) return '0';
    if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
    if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
    return String(n);
}

export function formatDuration(ms) {
    if (ms == null) return '-';
    if (ms < 1000) return ms + 'ms';
    return (ms / 1000).toFixed(1) + 's';
}

export function statusBadgeClass(status) {
    switch (status?.toLowerCase()) {
        case 'passed': case 'pass': case 'success': return 'badge-green';
        case 'failed': case 'fail': case 'error': return 'badge-red';
        case 'skipped': case 'skip': return 'badge-yellow';
        case 'running': case 'in_progress': return 'badge-blue';
        default: return 'badge-gray';
    }
}

// ---------- App ----------

import { Sidebar } from './components/sidebar.js';
import { Overview } from './pages/overview.js';
import { Servers } from './pages/servers.js';
import { Agents } from './pages/agents.js';
import { AgentDetail } from './pages/agent-detail.js';
import { Evals } from './pages/evals.js';
import { Sessions } from './pages/sessions.js';
import { SessionDetail } from './pages/session-detail.js';
import { Analytics } from './pages/analytics.js';
import { Tools } from './pages/tools.js';
import { LiveMonitor } from './pages/live-monitor.js';
import { ChatPlayground } from './pages/chat-playground.js';
import { LiveEval } from './pages/live-eval.js';

function App() {
    const route = useRoute();

    let page;
    let params;

    if (route === '/' || route === '') {
        page = html`<${Overview} />`;
    } else if (route === '/servers') {
        page = html`<${Servers} />`;
    } else if (route === '/agents') {
        page = html`<${Agents} />`;
    } else if ((params = matchRoute('/agents/:name', route))) {
        page = html`<${AgentDetail} name=${params.name} />`;
    } else if (route === '/evals') {
        page = html`<${Evals} />`;
    } else if (route === '/sessions') {
        page = html`<${Sessions} />`;
    } else if ((params = matchRoute('/sessions/:id', route))) {
        page = html`<${SessionDetail} id=${params.id} />`;
    } else if (route === '/analytics') {
        page = html`<${Analytics} />`;
    } else if (route === '/tools') {
        page = html`<${Tools} />`;
    } else if (route === '/monitor') {
        page = html`<${LiveMonitor} />`;
    } else if (route === '/chat') {
        page = html`<${ChatPlayground} />`;
    } else if (route === '/live-eval') {
        page = html`<${LiveEval} />`;
    } else {
        page = html`<div class="empty-state"><h2>Page not found</h2><p>${route}</p></div>`;
    }

    return html`
        <div class="app-layout">
            <${Sidebar} route=${route} />
            <main class="main-content">
                ${page}
            </main>
        </div>
    `;
}

// Mount
const root = createRoot(document.getElementById('root'));
root.render(html`<${App} />`);
