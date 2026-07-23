import { html } from '../app.js';

export function PageHeader({ title, subtitle }) {
    return html`
        <div class="page-header">
            <h1>${title}</h1>
            ${subtitle && html`<p>${subtitle}</p>`}
        </div>
    `;
}
