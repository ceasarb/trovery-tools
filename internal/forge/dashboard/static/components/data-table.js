import { html, useState, useMemo } from '../app.js';

// DataTable: a reusable sortable table component.
// columns: [{ key, label, render? }]
// data: array of row objects
// onRowClick: optional (row) => void
export function DataTable({ columns, data, onRowClick, emptyMessage }) {
    const [sortKey, setSortKey] = useState(null);
    const [sortDir, setSortDir] = useState('asc');

    const handleSort = (key) => {
        if (sortKey === key) {
            setSortDir(d => d === 'asc' ? 'desc' : 'asc');
        } else {
            setSortKey(key);
            setSortDir('asc');
        }
    };

    const sorted = useMemo(() => {
        if (!sortKey || !data) return data || [];
        return [...data].sort((a, b) => {
            const av = a[sortKey];
            const bv = b[sortKey];
            if (av == null && bv == null) return 0;
            if (av == null) return 1;
            if (bv == null) return -1;
            if (typeof av === 'number' && typeof bv === 'number') {
                return sortDir === 'asc' ? av - bv : bv - av;
            }
            const cmp = String(av).localeCompare(String(bv));
            return sortDir === 'asc' ? cmp : -cmp;
        });
    }, [data, sortKey, sortDir]);

    if (!data || data.length === 0) {
        return html`<div class="table-empty">${emptyMessage || 'No data'}</div>`;
    }

    const sortIndicator = (key) => {
        if (sortKey !== key) return '';
        return sortDir === 'asc' ? ' \u25B2' : ' \u25BC';
    };

    return html`
        <table class="data-table">
            <thead>
                <tr>
                    ${columns.map(col => html`
                        <th key=${col.key} onClick=${() => handleSort(col.key)}>
                            ${col.label}${sortIndicator(col.key)}
                        </th>
                    `)}
                </tr>
            </thead>
            <tbody>
                ${sorted.map((row, i) => html`
                    <tr key=${i}
                        class=${onRowClick ? 'clickable' : ''}
                        onClick=${onRowClick ? () => onRowClick(row) : null}>
                        ${columns.map(col => html`
                            <td key=${col.key}>
                                ${col.render ? col.render(row[col.key], row) : row[col.key] ?? '-'}
                            </td>
                        `)}
                    </tr>
                `)}
            </tbody>
        </table>
    `;
}
