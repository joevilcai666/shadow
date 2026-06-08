const API_BASE = '/api';

export async function fetchAPI<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || `API Error: ${res.status}`);
  }
  return res.json();
}

// Types
export interface Rule {
  id: string;
  content: string;
  scope: 'global' | 'project';
  project_path?: string;
  tags: string[];
  category: string;
  trigger_context: string;
  confidence: number;
  status: 'candidate' | 'active' | 'disabled' | 'conflicted';
  version: number;
  created_at: string;
  updated_at: string;
}

export interface Source {
  id: string;
  rule_id: string;
  signal_type: string;
  signal_strength: string;
  agent_name: string;
  project_path: string;
  raw_snippet: string;
  timestamp: string;
  confidence_contribution: number;
}

export interface Version {
  id: string;
  rule_id: string;
  version: number;
  content: string;
  diff: string;
  changed_by: string;
  change_reason: string;
  timestamp: string;
}

export interface Project {
  id: string;
  path: string;
  name: string;
  agents: string[];
  last_scan_at?: string;
  created_at: string;
}

export interface DashboardData {
  total_rules: number;
  active_rules: number;
  candidate_rules: number;
  disabled_rules: number;
  conflicted_rules: number;
  total_sources: number;
  project_count: number;
  agent_stats: Record<string, number>;
}

export interface Adapter {
  name: string;
  label: string;
  installed: boolean;
  enabled: boolean;
  target_path: string;
}

export interface Config {
  capture: { enabled: boolean; projects: Record<string, { enabled: boolean }> };
  privacy: { exclude_patterns: string[]; deny_patterns: string[] };
  distill: { threshold: string; auto_activate_low_risk: boolean; batch_mode: boolean };
  adapters: { claude_code: { enabled: boolean }; cursor: { enabled: boolean }; codex: { enabled: boolean } };
  server: { port: number; bind: string };
}

// API functions
export const api = {
  // Rules
  listRules: (params?: Record<string, string>) => {
    const qs = params ? '?' + new URLSearchParams(params).toString() : '';
    return fetchAPI<Rule[]>(`/rules${qs}`);
  },
  getRule: (id: string) => fetchAPI<Rule>(`/rules/${id}`),
  createRule: (data: Partial<Rule>) => fetchAPI<Rule>('/rules', { method: 'POST', body: JSON.stringify(data) }),
  updateRule: (id: string, data: Partial<Rule>) => fetchAPI<Rule>(`/rules/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteRule: (id: string) => fetchAPI<void>(`/rules/${id}`, { method: 'DELETE' }),
  getTimeline: (id: string) => fetchAPI<Source[]>(`/rules/${id}/timeline`),
  getVersions: (id: string) => fetchAPI<Version[]>(`/rules/${id}/versions`),
  rollback: (id: string, version: number) => fetchAPI<void>(`/rules/${id}/versions/${version}/rollback`, { method: 'PUT' }),
  batchRules: (action: string, ids: string[]) => fetchAPI<void>('/rules/batch', { method: 'POST', body: JSON.stringify({ action, ids }) }),

  // Projects
  listProjects: () => fetchAPI<Project[]>('/projects'),
  createProject: (data: Partial<Project>) => fetchAPI<Project>('/projects', { method: 'POST', body: JSON.stringify(data) }),

  // Dashboard
  getDashboard: () => fetchAPI<DashboardData>('/dashboard'),
  getDashboardMap: () => fetchAPI<{ nodes: any[]; edges: any[]; generated: number }>('/dashboard/map'),

  // Config
  getConfig: () => fetchAPI<Config>('/config'),
  updateConfig: (updates: Record<string, unknown>) => fetchAPI<void>('/config', { method: 'PUT', body: JSON.stringify(updates) }),

  // Adapters
  listAdapters: () => fetchAPI<Adapter[]>('/adapters'),
  toggleAdapter: (name: string, enabled: boolean) => fetchAPI<void>(`/adapters/${name}/toggle`, { method: 'POST', body: JSON.stringify({ enabled }) }),
  syncAdapters: () => fetchAPI<void>('/adapters/sync', { method: 'POST' }),

  // Capture
  captureStatus: () => fetchAPI<{ enabled: boolean }>('/capture/status'),
  toggleCapture: () => fetchAPI<void>('/capture/toggle', { method: 'POST' }),
};
