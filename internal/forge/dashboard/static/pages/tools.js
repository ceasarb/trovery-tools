import { html, useState, useEffect, api, apiPost } from '../app.js';
import { PageHeader } from '../components/layout.js';

export function Tools() {
    const [servers, setServers] = useState([]);
    const [selectedServer, setSelectedServer] = useState('');
    const [tools, setTools] = useState([]);
    const [selectedTool, setSelectedTool] = useState(null);
    const [toolsLoading, setToolsLoading] = useState(false);
    const [toolsError, setToolsError] = useState(null);
    const [args, setArgs] = useState('{}');
    const [result, setResult] = useState(null);
    const [calling, setCalling] = useState(false);
    const [callError, setCallError] = useState(null);

    // Fetch server list
    useEffect(() => {
        api('/servers').then(res => setServers(res.data || [])).catch(() => {});
    }, []);

    // Fetch tools when server changes
    useEffect(() => {
        if (!selectedServer) {
            setTools([]);
            setSelectedTool(null);
            return;
        }
        setToolsLoading(true);
        setToolsError(null);
        setSelectedTool(null);
        setResult(null);
        api('/tools?server=' + encodeURIComponent(selectedServer))
            .then(res => { setTools(res.data || []); setToolsLoading(false); })
            .catch(err => { setToolsError(err.message); setTools([]); setToolsLoading(false); });
    }, [selectedServer]);

    // Reset args when tool changes
    useEffect(() => {
        setResult(null);
        setCallError(null);
        if (selectedTool?.input_schema) {
            const schema = typeof selectedTool.input_schema === 'string'
                ? JSON.parse(selectedTool.input_schema)
                : selectedTool.input_schema;
            const defaults = buildDefaults(schema);
            setArgs(JSON.stringify(defaults, null, 2));
        } else {
            setArgs('{}');
        }
    }, [selectedTool]);

    const handleCall = async () => {
        setCalling(true);
        setCallError(null);
        setResult(null);
        try {
            const parsed = JSON.parse(args);
            const res = await apiPost('/tools/call', {
                server: selectedServer,
                tool: selectedTool.name,
                arguments: parsed,
            });
            setResult(res.data);
        } catch (err) {
            setCallError(err.message);
        }
        setCalling(false);
    };

    return html`
        <${PageHeader} title="Tool Playground" subtitle="Test MCP server tools interactively" />

        <div class="form-group" style="max-width: 300px; margin-bottom: 1.5rem;">
            <label>Server</label>
            <select value=${selectedServer} onChange=${(e) => setSelectedServer(e.target.value)}>
                <option value="">Select a server...</option>
                ${servers.map(s => html`<option key=${s.name} value=${s.name}>${s.name}</option>`)}
            </select>
        </div>

        ${!selectedServer ? html`
            <div class="empty-state">Select a server to see its tools</div>
        ` : toolsLoading ? html`
            <div class="loading">Starting server and listing tools...</div>
        ` : toolsError ? html`
            <div class="empty-state" style="color: var(--red);">Error: ${toolsError}</div>
        ` : html`
            <div class="playground-layout">
                <div class="tool-list">
                    ${tools.length === 0 ? html`
                        <div style="padding: 1rem; color: var(--text-muted);">No tools found</div>
                    ` : tools.map(t => html`
                        <div key=${t.name}
                             class="tool-list-item ${selectedTool?.name === t.name ? 'active' : ''}"
                             onClick=${() => setSelectedTool(t)}>
                            ${t.name}
                        </div>
                    `)}
                </div>

                <div class="tool-detail">
                    ${!selectedTool ? html`
                        <div class="empty-state">Select a tool from the list</div>
                    ` : html`
                        <h3 style="margin-bottom: 0.5rem;">${selectedTool.name}</h3>
                        ${selectedTool.description ? html`
                            <p class="tool-description">${selectedTool.description}</p>
                        ` : null}

                        ${selectedTool.input_schema ? html`
                            <${SchemaFields} schema=${selectedTool.input_schema} />
                        ` : null}

                        <div class="form-group">
                            <label>Arguments (JSON)</label>
                            <textarea
                                value=${args}
                                onChange=${(e) => setArgs(e.target.value)}
                                rows="6"
                            />
                        </div>

                        <button class="btn btn-primary" onClick=${handleCall} disabled=${calling}>
                            ${calling ? 'Calling...' : 'Call Tool'}
                        </button>

                        ${callError ? html`
                            <div class="result-panel">
                                <div class="tool-call-error" style="margin-top: 1rem;">${callError}</div>
                            </div>
                        ` : null}

                        ${result ? html`
                            <div class="result-panel">
                                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
                                    <span style="font-size: 0.8rem; color: var(--text-muted); text-transform: uppercase;">Result</span>
                                    <span class="badge badge-blue">${result.duration_ms}ms</span>
                                </div>
                                <pre>${JSON.stringify(result.result, null, 2)}</pre>
                            </div>
                        ` : null}
                    `}
                </div>
            </div>
        `}
    `;
}

// Display schema properties as reference (not editable form fields, just info)
function SchemaFields({ schema }) {
    const s = typeof schema === 'string' ? JSON.parse(schema) : schema;
    if (!s || !s.properties) return null;

    const props = Object.entries(s.properties);
    const required = s.required || [];

    return html`
        <div style="margin-bottom: 1rem; padding: 0.75rem; background: var(--bg-primary); border-radius: var(--radius); border: 1px solid var(--border);">
            <div style="font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-muted); margin-bottom: 0.5rem;">
                Input Schema
            </div>
            ${props.map(([name, prop]) => html`
                <div key=${name} style="margin-bottom: 0.35rem; font-size: 0.85rem;">
                    <code style="color: var(--accent-light);">${name}</code>
                    <span style="color: var(--text-muted); margin-left: 0.5rem;">${prop.type || 'any'}</span>
                    ${required.includes(name) ? html`<span class="badge badge-red" style="margin-left: 0.5rem; font-size: 0.65rem;">required</span>` : null}
                    ${prop.description ? html`<span style="color: var(--text-secondary); margin-left: 0.5rem;">\u2014 ${prop.description}</span>` : null}
                </div>
            `)}
        </div>
    `;
}

// Build default values from a JSON schema
function buildDefaults(schema) {
    if (!schema || !schema.properties) return {};
    const result = {};
    for (const [key, prop] of Object.entries(schema.properties)) {
        if (prop.default !== undefined) {
            result[key] = prop.default;
        } else if (prop.type === 'string') {
            result[key] = '';
        } else if (prop.type === 'number' || prop.type === 'integer') {
            result[key] = 0;
        } else if (prop.type === 'boolean') {
            result[key] = false;
        } else if (prop.type === 'array') {
            result[key] = [];
        } else if (prop.type === 'object') {
            result[key] = {};
        }
    }
    return result;
}
