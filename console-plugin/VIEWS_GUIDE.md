# App Dashboard Views Guide

This guide covers the four dashboard view modes available in the App Dashboard console plugin, along with search behavior, topology dependencies, keyboard tips, and practical usage guidance.

## Overview

The App Dashboard supports four complementary ways to explore your applications:

1. **App View** - visual card-based browsing by category
2. **Topology View** - dependency-focused graph visualization
3. **Namespace View** - swimlane-style grouping by namespace
4. **Compact View** - dense table layout for fast scanning

The header controls apply across all views:

- **View switcher** to move between the four layouts
- **Search input** to filter applications by display name, route name, namespace, or category
- **Filtered count** showing how many items are currently visible
- **Clear search** support from the search box
- **Show Private** toggle when a `Private` category exists

Search state and selected view mode are persisted in browser `localStorage`, so your last-used layout and filter remain active when you return.

---

## Shared Features Across All Views

### Search and filtering
The dashboard search applies to every view mode.

Search matches:

- Application display name
- Route or link name
- Namespace
- Category

Examples:

- `media`
- `ai`
- `octobox`
- `lab-namespace`

### Persistence
The following preferences are stored locally in the browser:

- selected view mode
- search query
- private visibility toggle
- expanded namespaces

### Data sources
The dashboard renders data from:

- OpenShift `Route` resources labeled with:
  - `dashboard.yamlwrangler.com/enabled=true`
- `ConfigMap` resources representing custom links labeled with:
  - `dashboard.yamlwrangler.com/type=custom-link`

### Empty states
If no matching applications are found, the dashboard now provides:

- clearer messaging
- search-aware guidance
- setup hints
- a link back to this guide

---

## View 1: App View

### What it is
App View is the default dashboard layout. Applications are grouped by category and displayed as cards.

### Best for
Use App View when you want:

- a visually friendly landing page
- quick access to application links
- easy browsing by category
- a balanced layout for small to medium app collections

### Features
- category sections with icons
- per-category app counts
- application cards with:
  - display name
  - namespace label or custom link label
  - description
  - quick external open action
- deployment links for routed apps
- hover polish for cards and links

### Typical usage
This is the best daily-driver view for operators who want a dashboard homepage experience.

### Screen description
You will see category headings such as Media, AI, Services, or Infrastructure, with cards beneath each section. Each card includes the app name, namespace badge, optional description, and an “Open Application” action.

---

## View 2: Topology View

### What it is
Topology View renders applications as nodes in a graph and shows relationships using dependency edges.

### Best for
Use Topology View when you want:

- to understand service relationships
- to visualize upstream/downstream dependencies
- to spot missing dependency targets
- to explain architecture to other users

### Features
- graph layout using PatternFly topology
- automatic node generation from dashboard apps
- dependency edges derived from annotations
- placeholder nodes for dependencies that are referenced but not found
- hover tooltips with:
  - app name
  - type
  - namespace
  - category
  - description
  - external-open hint
  - keyboard hint
- summary labels for:
  - category count
  - item count
  - dependency count
- click-to-open node behavior

### Node types
Topology nodes are rendered differently depending on type:

- **Application** - regular routed app
- **Custom link** - externally defined dashboard link
- **Missing dependency** - placeholder for a referenced app not currently present

### Screen description
You will see a graph canvas with nodes connected by directional edges. A stats bar appears above the graph, and an instruction bar appears below it. Hovering over a node displays more details; clicking a node opens the target URL in a new tab.

### When not to use it
Topology View is less ideal when:

- you have no dependency metadata yet
- you just want a simple launchpad
- you need a dense tabular overview

---

## View 3: Namespace View

### What it is
Namespace View organizes applications into expandable swimlanes by namespace.

### Best for
Use Namespace View when you want:

- to inspect ownership boundaries
- to compare workloads by namespace
- to see category distribution inside a namespace
- to work through application inventories in cluster-oriented workflows

### Features
- one section per namespace
- expandable namespace groups
- namespace color coding
- category breakdown chips within each namespace
- card layout for apps inside the namespace section
- persistence for expanded namespace state

### Screen description
Each namespace is shown as a labeled lane. Expanding a lane reveals all apps in that namespace, including descriptions and actions.

---

## View 4: Compact View

### What it is
Compact View presents the dashboard as an expandable, category-based table.

### Best for
Use Compact View when you want:

- the highest information density
- to scan many apps quickly
- a more operational inventory layout
- less visual spacing and more rows on screen

### Features
- expandable category sections
- compact PatternFly table layout
- columns for:
  - application
  - namespace
  - category
  - description
  - actions
- keyboard support on category expand/collapse controls

### Screen description
Categories appear as collapsible rows. Expanding a category reveals a compact table of applications with their metadata and actions.

---

## When to Use Each View

| View | Primary use case | Best for |
|---|---|---|
| App | Friendly dashboard homepage | Everyday browsing and launching |
| Topology | Relationship mapping | Dependency analysis and architecture review |
| Namespace | Cluster organization | Namespace ownership and operational grouping |
| Compact | Dense inventory scanning | Large app sets and quick audits |

### Suggested workflow
A practical way to use the dashboard:

1. Start in **App View** for quick launches
2. Switch to **Topology View** when troubleshooting service relationships
3. Use **Namespace View** to inspect cluster grouping
4. Use **Compact View** for audits and bulk scanning

---

## How to Add Applications to the Dashboard

### Routed applications
Label a route so it is included:

```yaml
metadata:
  labels:
    dashboard.yamlwrangler.com/enabled: "true"
  annotations:
    dashboard.yamlwrangler.com/display-name: "My App"
    dashboard.yamlwrangler.com/category: "Services"
    dashboard.yamlwrangler.com/description: "Internal service dashboard"
```

### Custom links
Create a labeled ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: docs-link
  namespace: lab-namespace
  labels:
    dashboard.yamlwrangler.com/type: custom-link
data:
  displayName: Documentation
  category: Services
  description: Team documentation portal
  url: https://docs.example.com
```

---

## How to Add Dependencies for Topology View

Topology dependencies are driven by the following annotation:

```yaml
dashboard.yamlwrangler.com/depends-on
```

### Format
Use a comma-separated list of application route names:

```yaml
metadata:
  annotations:
    dashboard.yamlwrangler.com/depends-on: "postgres,redis,api-gateway"
```

### Example
```yaml
metadata:
  name: octobox
  labels:
    dashboard.yamlwrangler.com/enabled: "true"
  annotations:
    dashboard.yamlwrangler.com/display-name: "Octobox"
    dashboard.yamlwrangler.com/category: "Services"
    dashboard.yamlwrangler.com/description: "GitHub inbox management"
    dashboard.yamlwrangler.com/depends-on: "postgres,redis"
```

### Important notes
- Dependency values should match dashboard app route names.
- If a dependency name is referenced but not found, the topology displays a placeholder node.
- Dependencies affect only Topology View; other views still display apps normally.
- Search filters apply before topology rendering, so filtered-out apps will not appear in the graph.

### Recommended practice
Use stable route names and keep dependency annotations short and accurate. This makes the graph easier to understand and reduces placeholder nodes.

---

## Keyboard Shortcuts and Navigation Tips

The dashboard includes improved accessibility support, but a few patterns are especially useful:

### General navigation
- `Tab` - move between header controls and actions
- `Shift + Tab` - move backward
- `Enter` or `Space` - activate expandable headers and buttons where supported

### Search
- Type in the search field to filter immediately
- Use the search clear button to reset results quickly

### Compact view
- Use `Tab` to focus a category header
- Press `Enter` or `Space` to expand or collapse the category

### Topology view
- Use page keyboard navigation first to reach the topology region
- Hover nodes for additional guidance
- Click nodes to open their target application

---

## Tooltips and Hints

Tooltips are used to make the interface easier to learn.

### Current tooltip usage includes
- view mode selector guidance
- topology node detail popovers
- topology summary badges
- open-in-new-tab hints

These hints are especially helpful for first-time users and for understanding what each view emphasizes.

---

## Tips and Tricks

### 1. Use search as a universal filter
Because search applies to all four views, you can:

- search in App View
- switch to Topology View
- immediately see a smaller relationship graph for only matching apps

This is especially useful when isolating one team, stack, or namespace.

### 2. Hide Private apps for shared demos
If your dashboard has a `Private` category, disable it before screen sharing or demos.

### 3. Use Namespace View for handoffs
When reviewing workloads with another operator, Namespace View is often the clearest representation of ownership and deployment boundaries.

### 4. Use Compact View for audits
Compact View is the fastest way to verify descriptions, namespaces, and app presence across a larger set of resources.

### 5. Placeholder nodes are useful
In Topology View, a gray placeholder node usually means one of these is true:

- the dependency target is not deployed
- the target app is not dashboard-enabled
- the dependency name does not match the route name exactly
- the current search filter removed the target from view

### 6. Prefer meaningful display names
Use `dashboard.yamlwrangler.com/display-name` to make all views more readable.

---

## Troubleshooting

### No apps are shown
Check that your routes are labeled:

```yaml
dashboard.yamlwrangler.com/enabled: "true"
```

### Custom links do not appear
Check that your ConfigMap is labeled:

```yaml
dashboard.yamlwrangler.com/type: custom-link
```

And verify that `data.url` is present.

### Topology has no edges
Check that dependencies are defined with:

```yaml
dashboard.yamlwrangler.com/depends-on
```

### Topology shows gray placeholder nodes
Verify that the dependency names exactly match the route names of the referenced apps.

### Search returns nothing
Try a broader term, clear the filter, or confirm that private apps are not hidden.

---

## Summary

Each view serves a different purpose:

- **App View** for everyday use
- **Topology View** for dependencies and architecture
- **Namespace View** for operational grouping
- **Compact View** for dense inventories

Use search to narrow scope, use dependencies to enrich topology, and use the view switcher to move between perspectives without changing the underlying data source.