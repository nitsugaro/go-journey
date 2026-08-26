import type { PlaceholderOr } from '../../../components/PlaceholderValueField'

export type HTTPRouteTableConfig = {
  default_http_instance?: string
  upstreams?: PlaceholderOr<Record<string, HTTPRouteUpstream>>
  routes?: PlaceholderOr<HTTPRoute[]>
  add_headers?: PlaceholderOr<Record<string, string>>
  strip_headers?: PlaceholderOr<string[]>
}

export type HTTPRouteUpstream = {
  url?: string
  http_instance?: string
  add_headers?: PlaceholderOr<Record<string, string>>
  strip_headers?: PlaceholderOr<string[]>
}

export type HTTPRoute = {
  name?: string
  uris?: PlaceholderOr<string[]>
  methods?: PlaceholderOr<string[]>
  upstream?: string
  http_instance?: string
  rewrite?: string
  policies?: PlaceholderOr<HTTPRoutePolicy[]>
  metadata?: PlaceholderOr<Record<string, unknown>>
  add_headers?: PlaceholderOr<Record<string, string>>
  strip_headers?: PlaceholderOr<string[]>
}

export type HTTPRoutePolicy = {
  name?: string
  type?: string
  config?: PlaceholderOr<Record<string, unknown>>
}

export const httpMethods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']

export const routePolicyTypes = [
  'header_present',
  'header_equals',
  'query_present',
  'query_equals',
  'method_allowed',
  'path_prefix',
  'host_equals',
]

export function parseRouteTableConfig(value: string): HTTPRouteTableConfig {
  const parsed = JSON.parse(value || '{}')
  return sanitizeRouteTableConfig(parsed)
}

export function sanitizeRouteTableConfig(value: unknown): HTTPRouteTableConfig {
  const input = isRecord(value) ? value : {}
  return {
    default_http_instance: asString(input.default_http_instance),
    upstreams: placeholderOr(input.upstreams, sanitizeUpstreams),
    routes: placeholderOr(input.routes, (routes) => Array.isArray(routes)
      ? routes.map(sanitizeRoute).filter(hasRouteURIs)
      : []),
    add_headers: placeholderOr(input.add_headers, stringMap),
    strip_headers: placeholderOr(input.strip_headers, stringArray),
  }
}

export function sanitizeRoute(value: unknown): HTTPRoute {
  const input = isRecord(value) ? value : {}
  return compact({
    name: asString(input.name),
    uris: placeholderOr(input.uris, stringArray),
    methods: placeholderOr(input.methods, (methods) => stringArray(methods).map((method) => method.toUpperCase())),
    upstream: asString(input.upstream),
    http_instance: asString(input.http_instance),
    rewrite: asString(input.rewrite),
    policies: placeholderOr(input.policies, (policies) => Array.isArray(policies) ? policies.map(sanitizePolicy) : []),
    metadata: placeholderOr(input.metadata, (metadata) => isRecord(metadata) ? metadata : {}),
    add_headers: placeholderOr(input.add_headers, stringMap),
    strip_headers: placeholderOr(input.strip_headers, stringArray),
  })
}

export function sanitizePolicy(value: unknown): HTTPRoutePolicy {
  const input = isRecord(value) ? value : {}
  return compact({
    name: asString(input.name),
    type: asString(input.type),
    config: placeholderOr(input.config, (config) => isRecord(config) ? config : {}),
  })
}

function sanitizeUpstreams(value: unknown): Record<string, HTTPRouteUpstream> {
  const input = isRecord(value) ? value : {}
  return Object.fromEntries(
    Object.entries(input)
      .map(([key, raw]) => [key, sanitizeUpstream(raw)] as const)
      .filter(([key]) => key.trim()),
  )
}

function sanitizeUpstream(value: unknown): HTTPRouteUpstream {
  const input = isRecord(value) ? value : {}
  return compact({
    url: asString(input.url),
    http_instance: asString(input.http_instance),
    add_headers: placeholderOr(input.add_headers, stringMap),
    strip_headers: placeholderOr(input.strip_headers, stringArray),
  })
}

function stringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.map((item) => String(item).trim()).filter(Boolean) : []
}

function stringMap(value: unknown): Record<string, string> {
  if (!isRecord(value)) return {}
  return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, String(item)]).filter(([key]) => key.trim()))
}

function asString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim() ? value : undefined
}

function placeholderOr<T>(value: unknown, sanitize: (value: unknown) => T): PlaceholderOr<T> {
  return typeof value === 'string' ? value : sanitize(value)
}

function hasRouteURIs(route: HTTPRoute) {
  return typeof route.uris === 'string' ? route.uris.trim().length > 0 : (route.uris?.length ?? 0) > 0
}

function compact<T extends Record<string, unknown>>(value: T): T {
  for (const key of Object.keys(value)) {
    const item = value[key]
    if (item === undefined || item === '' || (Array.isArray(item) && item.length === 0)) delete value[key]
    if (isRecord(item) && Object.keys(item).length === 0) delete value[key]
  }
  return value
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}
