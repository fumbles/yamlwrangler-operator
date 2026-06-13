import * as React from 'react';
import '@patternfly/react-topology/dist/esm/css/topology-components.css';
import '@patternfly/react-topology/dist/esm/css/topology-controlbar.css';
import '@patternfly/react-topology/dist/esm/css/topology-side-bar.css';
import '@patternfly/react-topology/dist/esm/css/topology-view.css';
import {
  TopologyView as PatternFlyTopologyView,
  Visualization,
  VisualizationProvider,
  VisualizationSurface,
  ModelKind,
  GraphComponent,
  withPanZoom,
  withSelection,
  DefaultNode,
  DefaultEdge,
  observer,
  ColaLayout,
  SELECTION_EVENT,
  action,
  type Model,
  type NodeModel,
  type EdgeModel,
  type ComponentFactory,
  type Node,
  type Graph,
  type EdgeStyle,
} from '@patternfly/react-topology';
import {
  Bullseye,
  EmptyState,
  EmptyStateBody,
  EmptyStateActions,
  Button,
  Label,
  Split,
  SplitItem,
  Title,
  Tooltip,
} from '@patternfly/react-core';
import {
  CubeIcon,
  LayerGroupIcon,
  TopologyIcon,
  ExternalLinkAltIcon,
  LinkIcon,
  QuestionCircleIcon,
} from '@patternfly/react-icons';

export interface AppRoute {
  name: string;
  namespace: string;
  displayName: string;
  category: string;
  description: string;
  url: string;
  isCustomLink?: boolean;
  annotations?: Record<string, string>;
  appGroup?: string; // For grouping related deployments
  subDeployments?: AppRoute[]; // Child deployments in a group
  isGrouped?: boolean; // True if this is a grouped app with sub-deployments
  customLinks?: { name: string; url: string; description?: string }[]; // Additional routes for sidecars/containers
}

export interface TopologyViewProps {
  groupedRoutes: Record<string, AppRoute[]>;
}

// ============================================================================
// Topology Data Structures
// ============================================================================

/**
 * Represents a node in the topology graph
 */
export interface TopologyNode {
  /** Unique identifier for the node */
  id: string;
  /** Type of node: 'application', 'custom-link', or 'service' */
  type: 'application' | 'custom-link' | 'service' | 'placeholder';
  /** Display label for the node */
  label: string;
  /** Additional data associated with the node */
  data: {
    /** Kubernetes namespace */
    namespace?: string;
    /** Application category */
    category?: string;
    /** URL to the application */
    url?: string;
    /** Description of the application */
    description?: string;
    /** Whether this is a custom link */
    isCustomLink?: boolean;
    /** Original route name */
    routeName?: string;
    /** Color for namespace badge */
    namespaceColor?: string;
  };
}

/**
 * Represents an edge (connection) between two nodes
 */
export interface TopologyEdge {
  /** Unique identifier for the edge */
  id: string;
  /** Source node ID */
  source: string;
  /** Target node ID */
  target: string;
  /** Edge styling options */
  edgeStyle?: {
    /** Edge type: 'solid', 'dashed', etc. */
    type?: string;
    /** Edge color */
    color?: string;
  };
}

/**
 * Complete topology data structure
 */
export interface TopologyData {
  /** Array of all nodes in the topology */
  nodes: TopologyNode[];
  /** Array of all edges connecting nodes */
  edges: TopologyEdge[];
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Generates a consistent color for a namespace based on its name
 * Uses a simple hash function to ensure the same namespace always gets the same color
 */
const getNamespaceColor = (namespace: string): string => {
  const colors = [
    '#0066cc', // blue
    '#3e8635', // green
    '#c9190b', // red
    '#f0ab00', // gold
    '#a18fff', // purple
    '#009596', // cyan
    '#ec7a08', // orange
    '#f4c145', // yellow
  ];

  // Simple hash function
  let hash = 0;
  for (let i = 0; i < namespace.length; i++) {
    hash = namespace.charCodeAt(i) + ((hash << 5) - hash);
  }

  return colors[Math.abs(hash) % colors.length];
};

/**
 * Detects dependencies from the app's annotations
 * Looks for the 'dashboard.yamlwrangler.com/depends-on' annotation
 *
 * @param app - The application route to check for dependencies
 * @returns Array of dependency IDs (app names)
 */
const detectDependencies = (app: AppRoute): string[] => {
  if (!app.annotations) {
    return [];
  }

  const dependsOnAnnotation = app.annotations['dashboard.yamlwrangler.com/depends-on'];

  if (!dependsOnAnnotation) {
    return [];
  }

  // Split by comma and trim whitespace
  return dependsOnAnnotation
    .split(',')
    .map((dep) => dep.trim())
    .filter((dep) => dep.length > 0);
};

/**
 * Creates a topology node from an AppRoute
 *
 * @param app - The application route to convert to a node
 * @returns A TopologyNode representing the application
 */
const createNodeFromApp = (app: AppRoute): TopologyNode => {
  const nodeType = app.isCustomLink ? 'custom-link' : 'application';

  return {
    id: app.name,
    type: nodeType,
    label: app.displayName || app.name,
    data: {
      namespace: app.namespace,
      category: app.category,
      url: app.url,
      description: app.description,
      isCustomLink: app.isCustomLink,
      routeName: app.name,
      namespaceColor: getNamespaceColor(app.namespace),
    },
  };
};

/**
 * Creates a placeholder node for a dependency that doesn't exist in the app list
 * This allows the topology to show dependencies even if the target app isn't deployed
 *
 * @param dependencyId - The ID of the missing dependency
 * @returns A placeholder TopologyNode
 */
const createPlaceholderNode = (dependencyId: string): TopologyNode => {
  return {
    id: dependencyId,
    type: 'placeholder',
    label: dependencyId,
    data: {
      description: 'Dependency not found in current applications',
    },
  };
};

/**
 * Creates an edge between two nodes
 *
 * @param source - Source node ID
 * @param target - Target node ID
 * @returns A TopologyEdge connecting the two nodes
 */
const createEdge = (source: string, target: string): TopologyEdge => {
  return {
    id: `${source}-to-${target}`,
    source,
    target,
    edgeStyle: {
      type: 'solid',
      color: '#8a8d90',
    },
  };
};

/**
 * Main transformation function that converts grouped routes into topology data
 *
 * This function:
 * 1. Creates nodes from all applications
 * 2. Detects dependencies from annotations
 * 3. Creates edges for dependencies
 * 4. Creates placeholder nodes for missing dependencies
 *
 * @param groupedRoutes - Routes grouped by category
 * @returns Complete topology data with nodes and edges
 */
const transformToTopologyData = (groupedRoutes: Record<string, AppRoute[]>): TopologyData => {
  const nodes: TopologyNode[] = [];
  const edges: TopologyEdge[] = [];
  const nodeIds = new Set<string>();
  const placeholderIds = new Set<string>();

  // First pass: Create nodes from all applications
  Object.values(groupedRoutes).forEach((routes) => {
    routes.forEach((app) => {
      const node = createNodeFromApp(app);
      nodes.push(node);
      nodeIds.add(node.id);
    });
  });

  // Second pass: Detect dependencies and create edges
  Object.values(groupedRoutes).forEach((routes) => {
    routes.forEach((app) => {
      const dependencies = detectDependencies(app);

      dependencies.forEach((depId) => {
        // Create edge from app to dependency
        edges.push(createEdge(app.name, depId));

        // If dependency doesn't exist in our node list, create a placeholder
        if (!nodeIds.has(depId) && !placeholderIds.has(depId)) {
          nodes.push(createPlaceholderNode(depId));
          placeholderIds.add(depId);
        }
      });
    });
  });

  return {
    nodes,
    edges,
  };
};

// ============================================================================
// Custom Node Component
// ============================================================================

/**
 * Custom node component with enhanced styling and tooltips
 */
const CustomNode: React.FC<{ element: Node }> = ({ element }) => {
  const data = element.getData() as TopologyNode['data'] & { type?: string; label?: string };
  const nodeType = data.type ?? 'application';

  let nodeColor = '#0066cc';
  let icon = <CubeIcon />;
  let nodeTypeLabel = 'Application';

  if (nodeType === 'custom-link') {
    nodeColor = '#f0ab00';
    icon = <LinkIcon />;
    nodeTypeLabel = 'Custom link';
  } else if (nodeType === 'placeholder') {
    nodeColor = '#d2d2d2';
    icon = <QuestionCircleIcon />;
    nodeTypeLabel = 'Missing dependency';
  } else if (data.namespaceColor) {
    nodeColor = data.namespaceColor;
  }

  const tooltipContent = (
    <div>
      <div>
        <strong>{data.label ?? element.getLabel()}</strong>
      </div>
      <div>Type: {nodeTypeLabel}</div>
      {data.namespace && <div>Namespace: {data.namespace}</div>}
      {data.category && <div>Category: {data.category}</div>}
      {data.description && <div>{data.description}</div>}
      {data.url && (
        <div style={{ marginTop: '8px' }}>
          <ExternalLinkAltIcon /> Click to open in a new tab
        </div>
      )}
      <div style={{ marginTop: '8px' }}>Keyboard: tab to the canvas, then pan and zoom</div>
    </div>
  );

  return (
    <Tooltip content={tooltipContent} position="top">
      <g aria-label={`${nodeTypeLabel}: ${data.label ?? element.getLabel()}`}>
        <DefaultNode
          element={element}
          showStatusDecorator={false}
          badge={data.namespace}
          badgeColor={nodeColor}
          badgeTextColor="#fff"
          badgeBorderColor={nodeColor}
        >
          <g transform="translate(-10, -10)">
            <foreignObject width="20" height="20">
              <div
                style={{
                  width: '20px',
                  height: '20px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: nodeColor,
                }}
                aria-hidden="true"
              >
                {icon}
              </div>
            </foreignObject>
          </g>
        </DefaultNode>
      </g>
    </Tooltip>
  );
};

// ============================================================================
// Component Factory
// ============================================================================

/**
 * Factory function to create topology components
 */
const componentFactory: ComponentFactory = (kind: ModelKind, type: string) => {
  switch (type) {
    case 'group':
      return withSelection()(DefaultNode);
    default:
      switch (kind) {
        case ModelKind.graph:
          return withPanZoom()(GraphComponent);
        case ModelKind.node:
          return withSelection()(observer(CustomNode) as React.ComponentType);
        case ModelKind.edge:
          return DefaultEdge;
        default:
          return undefined;
      }
  }
};

// ============================================================================
// TopologyView Component
// ============================================================================

const TopologyView: React.FC<TopologyViewProps> = ({ groupedRoutes }) => {
  const categoryCount = React.useMemo(() => Object.keys(groupedRoutes).length, [groupedRoutes]);
  const routeCount = React.useMemo(
    () => Object.values(groupedRoutes).reduce((total, routes) => total + routes.length, 0),
    [groupedRoutes],
  );

  const topologyData = React.useMemo(() => transformToTopologyData(groupedRoutes), [groupedRoutes]);

  const nodeLookup = React.useMemo(() => {
    return new Map(topologyData.nodes.map((node) => [node.id, node]));
  }, [topologyData.nodes]);

  const controller = React.useMemo(() => {
    const newController = new Visualization();
    newController.registerLayoutFactory(
      (type: string, graph: Graph): ColaLayout | undefined =>
        new ColaLayout(graph, {
          layoutOnDrag: false,
        }),
    );
    newController.registerComponentFactory(componentFactory);
    return newController;
  }, []);

  const model = React.useMemo<Model>(() => {
    const nodes: NodeModel[] = topologyData.nodes.map((node) => ({
      id: node.id,
      type: 'node',
      label: node.label,
      width: 100,
      height: 100,
      data: {
        ...node.data,
        type: node.type,
        label: node.label,
      },
    }));

    const edges: EdgeModel[] = topologyData.edges.map((edge) => ({
      id: edge.id,
      type: 'edge',
      source: edge.source,
      target: edge.target,
      edgeStyle: (edge.edgeStyle?.type ?? 'solid') as EdgeStyle,
    }));

    return {
      nodes,
      edges,
      graph: {
        id: 'topology-graph',
        type: 'graph',
        layout: 'Cola',
      },
    };
  }, [topologyData]);

  React.useEffect(() => {
    controller.fromModel(model, false);
    controller.getGraph().layout();
  }, [controller, model]);

  React.useEffect(() => {
    const handleSelection = action((ids: string[]) => {
      if (ids.length === 0) {
        return;
      }

      const selectedNode = nodeLookup.get(ids[0]);
      if (selectedNode?.data.url) {
        window.open(selectedNode.data.url, '_blank', 'noopener,noreferrer');
      }
    });

    controller.addEventListener(SELECTION_EVENT, handleSelection);
    return () => {
      controller.removeEventListener(SELECTION_EVENT, handleSelection);
    };
  }, [controller, nodeLookup]);

  if (topologyData.nodes.length === 0) {
    return (
      <PatternFlyTopologyView
        className="app-dashboard__topology-empty"
        style={{
          minHeight: '400px',
          borderRadius: '4px',
        }}
      >
        <Bullseye style={{ minHeight: '400px', padding: '1.5rem' }}>
          <EmptyState>
            <TopologyIcon
              className="app-dashboard__topology-empty-icon"
              style={{ fontSize: '3rem' }}
            />
            <Title headingLevel="h2" size="lg">
              No topology data available
            </Title>
            <EmptyStateBody>
              Add applications with the <code>dashboard.yamlwrangler.com/category</code> annotation
              and optional <code>dashboard.yamlwrangler.com/depends-on</code> dependencies to build
              the graph view.
            </EmptyStateBody>
            <EmptyStateBody>
              Helpful hint: search filters apply before the topology is rendered, so clearing the
              current filter may reveal hidden nodes.
            </EmptyStateBody>
            <EmptyStateActions>
              <Button
                component="a"
                variant="link"
                href="/api/plugins/app-dashboard-console-plugin/plugin-assets/VIEWS_GUIDE.md"
                target="_blank"
                rel="noopener noreferrer"
              >
                Open topology documentation
              </Button>
            </EmptyStateActions>
          </EmptyState>
        </Bullseye>
      </PatternFlyTopologyView>
    );
  }

  return (
    <div style={{ position: 'relative' }}>
      <div
        className="app-dashboard__topology-summary"
        style={{
          padding: '0.5rem 1rem',
        }}
      >
        <Split hasGutter>
          <SplitItem>
            <Tooltip content="Number of categories currently rendered in the topology">
              <Label color="blue" icon={<LayerGroupIcon />}>
                {categoryCount} {categoryCount === 1 ? 'category' : 'categories'}
              </Label>
            </Tooltip>
          </SplitItem>
          <SplitItem>
            <Tooltip content="Number of applications and custom links in the topology">
              <Label color="green" icon={<CubeIcon />}>
                {routeCount} {routeCount === 1 ? 'item' : 'items'}
              </Label>
            </Tooltip>
          </SplitItem>
          <SplitItem>
            <Tooltip content="Detected dependencies from dashboard.yamlwrangler.com/depends-on">
              <Label color="teal" icon={<TopologyIcon />}>
                {topologyData.edges.length}{' '}
                {topologyData.edges.length === 1 ? 'dependency' : 'dependencies'}
              </Label>
            </Tooltip>
          </SplitItem>
        </Split>
      </div>

      <PatternFlyTopologyView
        aria-label="Application topology visualization"
        className="app-dashboard__topology-canvas"
        style={{
          minHeight: '600px',
          outline: '2px solid transparent',
          outlineOffset: '2px',
        }}
      >
        <VisualizationProvider controller={controller}>
          <VisualizationSurface state={{}} />
        </VisualizationProvider>
      </PatternFlyTopologyView>

      <div
        className="app-dashboard__topology-footer"
        style={{
          padding: '0.5rem 1rem',
        }}
      >
        <strong>Tip:</strong> Click nodes to open applications • Drag to pan • Scroll to zoom •
        Hover for details • Dependencies shown with arrows • Keyboard shortcut hint: use Tab to move
        focus through page controls before interacting with the topology canvas
      </div>
    </div>
  );
};

export default TopologyView;

// Made with Bob
