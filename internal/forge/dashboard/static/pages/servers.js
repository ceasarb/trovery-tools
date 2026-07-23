import { html, useState, useEffect, api } from '../app.js';
import { PageHeader } from '../components/layout.js';
import { DataTable } from '../components/data-table.js';

const COLUMNS = [
    { key: 'name', label: 'Name' },
    { key: 'language', label: 'Language', render: (v) => html`<span class="badge badge-blue">${v}</span>` },
    { key: 'transport', label: 'Transport', render: (v) => html`<span class="badge badge-purple">${v}</span>` },
    { key: 'path', label: 'Path' },
];

export function Servers() {
    const [data, setData] = useState(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        api('/servers').then(res => { setData(res.data); setLoading(false); })
            .catch(() => { setData([]); setLoading(false); });
    }, []);

    if (loading) return html`<div class="loading">Loading...</div>`;

    return html`
        <${PageHeader} title="Servers" subtitle="MCP servers in this workspace" />
        <${DataTable} columns=${COLUMNS} data=${data} emptyMessage="No servers found. Run ajnt server create to scaffold one." />
    `;
}
