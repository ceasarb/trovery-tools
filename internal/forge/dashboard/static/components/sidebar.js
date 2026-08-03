import { html, navigate } from '../app.js';

const NAV_ITEMS = [
    { path: '/',          icon: '\u25A3', label: 'Overview' },
    { path: '/servers',   icon: '\u2756', label: 'Servers' },
    { path: '/agents',    icon: '\u2726', label: 'Agents' },
    { path: '/evals',     icon: '\u2714', label: 'Evals' },
    { path: '/sessions',  icon: '\u25C9', label: 'Sessions' },
    { path: '/analytics', icon: '\u2261', label: 'Analytics' },
    { path: '/tools',     icon: '\u2692', label: 'Tools' },
    { path: '/monitor',   icon: '\u25C8', label: 'Live Monitor' },
    { path: '/chat',      icon: '\u2709', label: 'Chat' },
    { path: '/live-eval', icon: '\u25B6', label: 'Live Eval' },
];

export function Sidebar({ route }) {
    const isActive = (path) => {
        if (path === '/') return route === '/' || route === '';
        return route.startsWith(path);
    };

    return html`
        <aside class="sidebar">
            <div class="sidebar-logo">
                Trovery Forge
                <span>Dashboard</span>
            </div>
            <nav class="sidebar-nav">
                ${NAV_ITEMS.map(item => html`
                    <a key=${item.path}
                       class="nav-item ${isActive(item.path) ? 'active' : ''}"
                       href="#${item.path}"
                       onClick=${(e) => { e.preventDefault(); navigate(item.path); }}>
                        <span class="nav-icon">${item.icon}</span>
                        ${item.label}
                    </a>
                `)}
            </nav>
        </aside>
    `;
}
