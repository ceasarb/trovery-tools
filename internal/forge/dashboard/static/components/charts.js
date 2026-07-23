import Chart from 'chart.js/auto';

// Shared dark-theme defaults for Chart.js
const CHART_COLORS = ['#7c3aed', '#3b82f6', '#22c55e', '#eab308', '#ef4444', '#ec4899'];

const DARK_DEFAULTS = {
    color: '#9ca3af',
    borderColor: '#2d2d52',
    backgroundColor: 'transparent',
};

function baseOptions(title) {
    return {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
            legend: {
                labels: { color: '#9ca3af', font: { size: 11 } },
            },
            title: title ? {
                display: true,
                text: title,
                color: '#e5e5ef',
                font: { size: 13, weight: '600' },
            } : { display: false },
        },
        scales: {
            x: {
                ticks: { color: '#6b7280', font: { size: 10 } },
                grid: { color: '#1e1e36' },
            },
            y: {
                ticks: { color: '#6b7280', font: { size: 10 } },
                grid: { color: '#1e1e36' },
            },
        },
    };
}

// Track chart instances to destroy before re-creating
const chartInstances = {};

function destroyExisting(canvasId) {
    if (chartInstances[canvasId]) {
        chartInstances[canvasId].destroy();
        delete chartInstances[canvasId];
    }
}

export function renderLineChart(canvasId, labels, datasets, title) {
    destroyExisting(canvasId);
    const ctx = document.getElementById(canvasId);
    if (!ctx) return;

    const ds = datasets.map((d, i) => ({
        label: d.label,
        data: d.data,
        borderColor: d.color || CHART_COLORS[i % CHART_COLORS.length],
        backgroundColor: (d.color || CHART_COLORS[i % CHART_COLORS.length]) + '22',
        tension: 0.3,
        fill: datasets.length === 1,
        pointRadius: 3,
        pointHoverRadius: 5,
    }));

    chartInstances[canvasId] = new Chart(ctx, {
        type: 'line',
        data: { labels, datasets: ds },
        options: baseOptions(title),
    });
}

export function renderBarChart(canvasId, labels, data, title, horizontal) {
    destroyExisting(canvasId);
    const ctx = document.getElementById(canvasId);
    if (!ctx) return;

    const colors = labels.map((_, i) => CHART_COLORS[i % CHART_COLORS.length]);
    const opts = baseOptions(title);

    if (horizontal) {
        opts.indexAxis = 'y';
    }

    chartInstances[canvasId] = new Chart(ctx, {
        type: 'bar',
        data: {
            labels,
            datasets: [{
                data,
                backgroundColor: colors.map(c => c + '88'),
                borderColor: colors,
                borderWidth: 1,
            }],
        },
        options: {
            ...opts,
            plugins: {
                ...opts.plugins,
                legend: { display: false },
            },
        },
    });
}
