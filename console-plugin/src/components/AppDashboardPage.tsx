import * as React from 'react';
import {
  DocumentTitle,
  ListPageHeader,
  useK8sWatchResource,
} from '@openshift-console/dynamic-plugin-sdk';
import type { AppRoute } from './TopologyView';
import {
  PageSection,
  Card,
  CardBody,
  Button,
  Gallery,
  Spinner,
  EmptyState,
  EmptyStateBody,
  Title,
  Label,
  Flex,
  FlexItem,
  Split,
  SplitItem,
  Switch,
  ToggleGroup,
  ToggleGroupItem,
  Truncate,
  SearchInput,
  Tooltip,
} from '@patternfly/react-core';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';
import {
  ExternalLinkAltIcon,
  CubesIcon,
  LayerGroupIcon,
  CubeIcon,
  LinkIcon,
  EyeIcon,
  EyeSlashIcon,
  SearchIcon,
  InfrastructureIcon,
  UnlinkIcon,
  FileVideoIcon,
  PrivateIcon,
  ServicesIcon,
  ThIcon,
  StreamIcon,
  ThLargeIcon,
  AngleDownIcon,
  AngleRightIcon,
  StarIcon,
  OutlinedStarIcon,
} from '@patternfly/react-icons';

import './example.css';

// Deployment type definition
interface Deployment {
  metadata: {
    name: string;
    namespace: string;
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  spec: {
    replicas?: number;
    selector?: {
      matchLabels?: Record<string, string>;
    };
  };
  status?: {
    availableReplicas?: number;
    replicas?: number;
  };
}

// Route type definition
interface Route {
  metadata: {
    name: string;
    namespace: string;
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  spec: {
    host?: string;
    tls?: {
      termination?: string;
    };
    to?: {
      kind?: string;
      name?: string;
    };
  };
}

// ConfigMap type definition
interface ConfigMap {
  metadata: {
    name: string;
    namespace: string;
    labels?: Record<string, string>;
  };
  data?: Record<string, string>;
}

// View mode type definition
type ViewMode = 'app' | 'topology' | 'namespace' | 'compact';

const PRIVATE_CATEGORY = 'Private';
const SHOW_PRIVATE_KEY = 'dashboard.showPrivate';
const VIEW_MODE_KEY = 'dashboard.viewMode';
const SEARCH_QUERY_KEY = 'dashboard.searchQuery';
const FAVORITES_KEY = 'dashboard.favorites';
const COLLAPSED_NAMESPACES_KEY = 'dashboard.collapsedNamespaces';
const DOCS_PATH = '/api/plugins/app-dashboard-console-plugin/plugin-assets/VIEWS_GUIDE.md';
const DASHBOARD_ENABLED_KEY = 'dashboard.yamlwrangler.com/enabled';

// Map category names to PatternFly icons
const getCategoryIcon = (category: string) => {
  const categoryLower = category.toLowerCase();

  if (categoryLower.includes('ai')) {
    return <SearchIcon />;
  }
  if (categoryLower.includes('infra')) {
    return <InfrastructureIcon />;
  }
  if (categoryLower.includes('link')) {
    return <UnlinkIcon />;
  }
  if (categoryLower.includes('media')) {
    return <FileVideoIcon />;
  }
  if (categoryLower.includes('private')) {
    return <PrivateIcon />;
  }
  if (categoryLower.includes('service') || categoryLower.includes('cloud')) {
    return <ServicesIcon />;
  }

  // Default icon
  return <LayerGroupIcon />;
};

// Assign consistent colors to namespaces
const getNamespaceColor = (
  namespace: string,
): 'purple' | 'blue' | 'green' | 'orange' | 'red' | 'teal' | 'grey' => {
  // Hash the namespace string to get a consistent color
  let hash = 0;
  for (let i = 0; i < namespace.length; i++) {
    hash = namespace.charCodeAt(i) + ((hash << 5) - hash);
  }

  const colors: ('purple' | 'blue' | 'green' | 'orange' | 'red' | 'teal' | 'grey')[] = [
    'purple',
    'blue',
    'green',
    'orange',
    'teal',
    'red',
    'grey',
  ];

  const index = Math.abs(hash) % colors.length;
  return colors[index];
};

const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error) {
    return error.message;
  }

  if (typeof error === 'string') {
    return error;
  }

  try {
    return JSON.stringify(error);
  } catch {
    return 'Unknown error';
  }
};

const AppDashboardPage: React.FC = () => {
  // State for view mode with localStorage persistence
  const [viewMode, setViewMode] = React.useState<ViewMode>(() => {
    const stored = localStorage.getItem(VIEW_MODE_KEY);
    return stored === 'topology' ||
      stored === 'namespace' ||
      stored === 'compact' ||
      stored === 'app'
      ? stored
      : 'compact';
  });

  // State for showing/hiding private category
  const [showPrivate, setShowPrivate] = React.useState<boolean>(() => {
    const stored = localStorage.getItem(SHOW_PRIVATE_KEY);
    return stored === 'true';
  });

  const [searchQuery, setSearchQuery] = React.useState<string>(() => {
    return localStorage.getItem(SEARCH_QUERY_KEY) ?? '';
  });

  // State for favorites with localStorage persistence
  const [favorites, setFavorites] = React.useState<Set<string>>(() => {
    const stored = localStorage.getItem(FAVORITES_KEY);
    if (!stored) {
      return new Set();
    }
    try {
      const parsed = JSON.parse(stored) as unknown;
      return Array.isArray(parsed)
        ? new Set(parsed.filter((value): value is string => typeof value === 'string'))
        : new Set();
    } catch {
      return new Set();
    }
  });

  // State for collapsed rows in compact view. Empty means all categories are expanded.
  const [collapsedCategories, setCollapsedCategories] = React.useState<Set<string>>(new Set());

  // State for expandable app groups (track which grouped apps are expanded)
  const [expandedAppGroups, setExpandedAppGroups] = React.useState<Set<string>>(new Set());

  // State for collapsed namespaces in namespace view. Empty means all namespaces are expanded.
  const [collapsedNamespaces, setCollapsedNamespaces] = React.useState<Set<string>>(() => {
    const stored = localStorage.getItem(COLLAPSED_NAMESPACES_KEY);
    if (!stored) {
      return new Set();
    }

    try {
      const parsed = JSON.parse(stored) as unknown;
      return Array.isArray(parsed)
        ? new Set(parsed.filter((value): value is string => typeof value === 'string'))
        : new Set();
    } catch {
      return new Set();
    }
  });

  // Watch deployments broadly so operator-owned workloads that preserve annotations
  // but reconcile labels away can still appear on the dashboard.
  const deploymentsWatch = useK8sWatchResource<Deployment[]>({
    groupVersionKind: {
      group: 'apps',
      version: 'v1',
      kind: 'Deployment',
    },
    isList: true,
  });
  const [deployments, deploymentsLoaded] = deploymentsWatch as [Deployment[], boolean, unknown];

  // Watch all routes (no label filter) to find routes for deployments
  const routesWatch = useK8sWatchResource<Route[]>({
    groupVersionKind: {
      group: 'route.openshift.io',
      version: 'v1',
      kind: 'Route',
    },
    isList: true,
  });
  const [routes, routesLoaded, routesError] = routesWatch as [Route[], boolean, unknown];

  // Watch ConfigMaps with dashboard label for custom links
  const configMapsWatch = useK8sWatchResource<ConfigMap[]>({
    groupVersionKind: {
      version: 'v1',
      kind: 'ConfigMap',
    },
    isList: true,
    selector: {
      matchLabels: {
        'dashboard.yamlwrangler.com/type': 'custom-link',
      },
    },
  });
  const [configMaps, configMapsLoaded, configMapsError] = configMapsWatch as [
    ConfigMap[],
    boolean,
    unknown,
  ];

  // Save view mode preference to localStorage when it changes
  React.useEffect(() => {
    localStorage.setItem(VIEW_MODE_KEY, viewMode);
  }, [viewMode]);

  // Save show private preference to localStorage when it changes
  React.useEffect(() => {
    localStorage.setItem(SHOW_PRIVATE_KEY, String(showPrivate));
  }, [showPrivate]);

  // Save favorites to localStorage when they change
  React.useEffect(() => {
    localStorage.setItem(FAVORITES_KEY, JSON.stringify(Array.from(favorites)));
  }, [favorites]);

  // Save collapsed namespaces to localStorage when they change
  React.useEffect(() => {
    localStorage.setItem(COLLAPSED_NAMESPACES_KEY, JSON.stringify(Array.from(collapsedNamespaces)));
  }, [collapsedNamespaces]);

  React.useEffect(() => {
    localStorage.setItem(SEARCH_QUERY_KEY, searchQuery);
  }, [searchQuery]);

  // Toggle favorite status for an app
  const toggleFavorite = React.useCallback((appKey: string) => {
    setFavorites((prev) => {
      const newFavorites = new Set(prev);
      if (newFavorites.has(appKey)) {
        newFavorites.delete(appKey);
      } else {
        newFavorites.add(appKey);
      }
      return newFavorites;
    });
  }, []);

  // Check if an app is favorited
  const isFavorite = React.useCallback(
    (namespace: string, name: string) => {
      return favorites.has(`${namespace}/${name}`);
    },
    [favorites],
  );

  // Process deployments and configmaps into grouped structure
  const groupedRoutes = React.useMemo(() => {
    const grouped: Record<string, AppRoute[]> = {};

    // Process deployments
    if (deploymentsLoaded && routesLoaded) {
      // First, create AppRoute objects for all deployments
      const allDeploymentApps: AppRoute[] = [];

      deployments.forEach((deployment) => {
        const dashboardEnabled =
          deployment.metadata.labels?.[DASHBOARD_ENABLED_KEY] === 'true' ||
          deployment.metadata.annotations?.[DASHBOARD_ENABLED_KEY] === 'true';
        if (!dashboardEnabled) {
          return;
        }

        const namespace = deployment.metadata.namespace;
        const deploymentName = deployment.metadata.name;
        const displayName =
          deployment.metadata.annotations?.['dashboard.yamlwrangler.com/display-name'] ??
          deploymentName;
        const category =
          deployment.metadata.annotations?.['dashboard.yamlwrangler.com/category'] ??
          'Uncategorized';
        const description =
          deployment.metadata.annotations?.['dashboard.yamlwrangler.com/description'] ?? '';
        const appGroup = deployment.metadata.annotations?.['dashboard.yamlwrangler.com/app-group'];

        // Look for a route that points to this deployment
        const primaryRouteName =
          deployment.metadata.annotations?.['dashboard.yamlwrangler.com/primary-route'];

        let matchedRoute: Route | undefined;

        if (primaryRouteName) {
          matchedRoute = routes.find(
            (r) => r.metadata.namespace === namespace && r.metadata.name === primaryRouteName,
          );
        }

        matchedRoute ??= routes.find((r) => {
          if (r.metadata.namespace !== namespace) return false;

          const routeTarget = r.spec.to?.name;
          const routeName = r.metadata.name;

          // Direct match: route target or name matches deployment name
          if (routeTarget === deploymentName || routeName === deploymentName) return true;

          // Handle deployment suffix: if deployment is "plane-admin-wl", also match route target "plane-admin"
          // This handles cases where deployments have suffixes like -wl, -deployment, etc.
          const deploymentBase = deploymentName.replace(/-wl$|-deployment$/, '');
          if (routeTarget === deploymentBase || routeName === deploymentBase) return true;

          return false;
        });

        // Build the URL if a route was found
        let url = '';
        if (matchedRoute) {
          const protocol = matchedRoute.spec.tls ? 'https' : 'http';
          const host = matchedRoute.spec.host ?? '';
          url = `${protocol}://${host}`;
        }

        // Parse customLinks from annotation
        let customLinks: { name: string; url: string }[] | undefined;
        const customLinksAnnotation =
          deployment.metadata.annotations?.['dashboard.yamlwrangler.com/custom-links'];
        if (customLinksAnnotation) {
          try {
            customLinks = JSON.parse(customLinksAnnotation) as { name: string; url: string }[];
          } catch (e) {
            console.warn(`Failed to parse custom-links for ${deploymentName}:`, e);
          }
        }

        const appRoute: AppRoute = {
          name: deploymentName,
          namespace,
          displayName,
          category,
          description,
          url,
          isCustomLink: false,
          appGroup,
          annotations: deployment.metadata.annotations,
          customLinks,
        };

        allDeploymentApps.push(appRoute);
      });

      // Group deployments by app-group annotation
      const appGroups: Record<string, AppRoute[]> = {};
      const standaloneApps: AppRoute[] = [];

      allDeploymentApps.forEach((app) => {
        if (app.appGroup) {
          const groupKey = `${app.namespace}/${app.appGroup}`;
          appGroups[groupKey] ??= [];
          appGroups[groupKey].push(app);
        } else {
          standaloneApps.push(app);
        }
      });

      // Create grouped app entries
      Object.entries(appGroups).forEach(([, apps]) => {
        if (apps.length === 1) {
          // Only one deployment in group, treat as standalone
          const app = apps[0];
          grouped[app.category] ??= [];
          grouped[app.category].push(app);
        } else {
          // Multiple deployments in group, create a grouped entry
          // Find the main app: the one that has the primary-route annotation
          const mainApp =
            apps.find((a) => a.annotations?.['dashboard.yamlwrangler.com/primary-route']) ??
            apps.find((a) => a.url) ??
            apps[0];

          const category = mainApp.category;
          const appGroupName = mainApp.appGroup ?? 'Unknown';

          const groupedApp: AppRoute = {
            name: appGroupName,
            namespace: mainApp.namespace,
            displayName: mainApp.displayName,
            category,
            description: `${String(apps.length)} deployments`,
            url: mainApp.url,
            isCustomLink: false,
            appGroup: mainApp.appGroup,
            subDeployments: apps,
            isGrouped: true,
          };

          grouped[category] ??= [];
          grouped[category].push(groupedApp);
        }
      });

      // Add standalone apps
      standaloneApps.forEach((app) => {
        grouped[app.category] ??= [];
        grouped[app.category].push(app);
      });
    }

    // Process custom links from ConfigMaps
    if (configMapsLoaded) {
      configMaps.forEach((cm) => {
        const data = cm.data ?? {};
        const displayName = data.displayName;
        const category = data.category;
        const description = data.description;
        const url = data.url;
        const namespace = cm.metadata.namespace;

        if (url) {
          const appRoute: AppRoute = {
            name: cm.metadata.name,
            namespace,
            displayName,
            category,
            description,
            url,
            isCustomLink: true,
          };

          grouped[category] ??= [];
          grouped[category].push(appRoute);
        }
      });
    }

    // Sort routes within each category: favorites first, then alphabetically by display name
    Object.keys(grouped).forEach((cat) => {
      grouped[cat].sort((a, b) => {
        const aFav = favorites.has(`${a.namespace}/${a.name}`);
        const bFav = favorites.has(`${b.namespace}/${b.name}`);

        // Favorites come first
        if (aFav && !bFav) return -1;
        if (!aFav && bFav) return 1;

        // Within same favorite status, sort alphabetically
        return a.displayName.localeCompare(b.displayName, undefined, { sensitivity: 'base' });
      });
    });

    return grouped;
  }, [
    deployments,
    deploymentsLoaded,
    routes,
    routesLoaded,
    configMaps,
    configMapsLoaded,
    favorites,
  ]);

  const normalizedSearchQuery = React.useMemo(
    () => searchQuery.trim().toLowerCase(),
    [searchQuery],
  );

  const visibleRoutes = React.useMemo(() => {
    const apps = Object.values(groupedRoutes).flat();

    return apps.filter((app) => {
      if (!showPrivate && app.category === PRIVATE_CATEGORY) {
        return false;
      }

      if (!normalizedSearchQuery) {
        return true;
      }

      return [app.displayName, app.name, app.namespace, app.category].some((value) =>
        value.toLowerCase().includes(normalizedSearchQuery),
      );
    });
  }, [groupedRoutes, showPrivate, normalizedSearchQuery]);

  const filteredGroupedRoutes = React.useMemo(() => {
    const grouped: Record<string, AppRoute[]> = {};

    visibleRoutes.forEach((app) => {
      grouped[app.category] ??= [];
      grouped[app.category].push(app);
    });

    Object.keys(grouped).forEach((category) => {
      grouped[category].sort((a, b) => {
        const aFav = favorites.has(`${a.namespace}/${a.name}`);
        const bFav = favorites.has(`${b.namespace}/${b.name}`);

        // Favorites come first
        if (aFav && !bFav) return -1;
        if (!aFav && bFav) return 1;

        // Within same favorite status, sort alphabetically
        return a.displayName.localeCompare(b.displayName, undefined, { sensitivity: 'base' });
      });
    });

    return grouped;
  }, [visibleRoutes, favorites]);

  // Filter out Private category if showPrivate is false
  const filteredCategories = React.useMemo(() => {
    return Object.keys(filteredGroupedRoutes).sort();
  }, [filteredGroupedRoutes]);

  // Check if Private category exists
  const hasPrivateCategory = React.useMemo(() => {
    return Object.keys(groupedRoutes).includes(PRIVATE_CATEGORY);
  }, [groupedRoutes]);

  const totalVisibleCount = visibleRoutes.length;
  const totalAppCount = React.useMemo(() => {
    return Object.values(groupedRoutes).reduce((total, apps) => {
      return total + apps.filter((app) => showPrivate || app.category !== PRIVATE_CATEGORY).length;
    }, 0);
  }, [groupedRoutes, showPrivate]);

  const groupedByNamespace = React.useMemo(() => {
    const grouped: Record<string, AppRoute[]> = {};

    visibleRoutes.forEach((app) => {
      const namespace = app.isCustomLink ? 'Custom Links' : app.namespace;
      grouped[namespace] ??= [];
      grouped[namespace].push(app);
    });

    Object.keys(grouped).forEach((ns) => {
      grouped[ns].sort((a, b) => {
        const aFav = favorites.has(`${a.namespace}/${a.name}`);
        const bFav = favorites.has(`${b.namespace}/${b.name}`);

        // Favorites come first
        if (aFav && !bFav) return -1;
        if (!aFav && bFav) return 1;

        // Within same favorite status, sort alphabetically
        return a.displayName.localeCompare(b.displayName, undefined, { sensitivity: 'base' });
      });
    });

    return grouped;
  }, [visibleRoutes, favorites]);

  const getDeploymentLink = (namespace: string, routeName: string) => {
    // Try common deployment name patterns
    const possibleNames = [
      routeName,
      routeName.replace(/-route$/, ''),
      routeName.replace(/-svc$/, ''),
    ];
    // Link to deployments list filtered by app name
    return `/k8s/ns/${namespace}/deployments?name=${possibleNames[0]}`;
  };

  const renderContent = () => {
    const isLoading = !routesLoaded || !configMapsLoaded;
    const hasError = Boolean(routesError) || Boolean(configMapsError);

    if (isLoading) {
      return (
        <PageSection>
          <Spinner size="lg" />
          <p>Loading applications...</p>
        </PageSection>
      );
    }

    if (hasError) {
      return (
        <PageSection>
          <EmptyState>
            <CubesIcon />
            <Title headingLevel="h4" size="lg">
              Error loading dashboard
            </Title>
            <EmptyStateBody>
              {routesError && <p>Routes: {getErrorMessage(routesError)}</p>}
              {configMapsError && <p>ConfigMaps: {getErrorMessage(configMapsError)}</p>}
            </EmptyStateBody>
          </EmptyState>
        </PageSection>
      );
    }

    if (filteredCategories.length === 0) {
      return (
        <PageSection>
          <EmptyState>
            <CubesIcon />
            <Title headingLevel="h4" size="lg">
              {normalizedSearchQuery
                ? 'No applications match your search'
                : 'No applications found'}
            </Title>
            <EmptyStateBody>
              {normalizedSearchQuery ? (
                <>
                  <p>
                    No applications matched <code>{searchQuery}</code>. Try searching by application
                    name, namespace, or category.
                  </p>
                  <p>
                    Clear the current search to see all available applications, or add more
                    dashboard-enabled deployments and custom links.
                  </p>
                </>
              ) : (
                <>
                  <p>
                    No deployments are labeled or annotated with{' '}
                    <code>dashboard.yamlwrangler.com/enabled=true</code>
                  </p>
                  <p>
                    No ConfigMaps are labeled with{' '}
                    <code>dashboard.yamlwrangler.com/type=custom-link</code>
                  </p>
                  <p>
                    Add the dashboard labels or review the usage guide for setup examples and
                    topology dependency annotations.
                  </p>
                </>
              )}
              <p>
                <a href={DOCS_PATH} target="_blank" rel="noopener noreferrer">
                  Open dashboard views guide
                </a>
              </p>
            </EmptyStateBody>
          </EmptyState>
        </PageSection>
      );
    }

    // Render content based on selected view mode
    if (viewMode === 'namespace') {
      // Toggle namespace collapse
      const toggleNamespace = (namespace: string) => {
        setCollapsedNamespaces((prev) => {
          const newSet = new Set(prev);
          if (newSet.has(namespace)) {
            newSet.delete(namespace);
          } else {
            newSet.add(namespace);
          }
          return newSet;
        });
      };

      // Get sorted namespaces
      const namespaces = Object.keys(groupedByNamespace).sort();

      return (
        <>
          {namespaces.map((namespace) => {
            const isExpanded = !collapsedNamespaces.has(namespace);
            const apps = groupedByNamespace[namespace];
            const namespaceColor =
              namespace === 'Custom Links' ? 'blue' : getNamespaceColor(namespace);

            // Get category breakdown for this namespace
            const categoryCount: Record<string, number> = {};
            apps.forEach((app) => {
              categoryCount[app.category] = (categoryCount[app.category] || 0) + 1;
            });

            return (
              <PageSection
                key={namespace}
                style={{
                  paddingTop: '0.5rem',
                  paddingBottom: '0.5rem',
                }}
              >
                {/* Swimlane Container */}
                <div
                  className="app-dashboard__namespace-lane"
                  style={{
                    borderLeft: `4px solid var(--pf-v5-global--palette--${namespaceColor}-300)`,
                  }}
                >
                  {/* Swimlane Header - Clickable */}
                  <div
                    className={`app-dashboard__namespace-header${
                      isExpanded ? '' : ' app-dashboard__namespace-header--collapsed'
                    }`}
                    onClick={() => {
                      toggleNamespace(namespace);
                    }}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        toggleNamespace(namespace);
                      }
                    }}
                    role="button"
                    tabIndex={0}
                  >
                    <Flex
                      alignItems={{ default: 'alignItemsCenter' }}
                      spaceItems={{ default: 'spaceItemsSm' }}
                    >
                      {/* Expand/Collapse Icon */}
                      <FlexItem className="app-dashboard__toggle-icon">
                        {isExpanded ? <AngleDownIcon /> : <AngleRightIcon />}
                      </FlexItem>

                      {/* Namespace Badge */}
                      <FlexItem>
                        <Label
                          color={namespaceColor}
                          icon={namespace === 'Custom Links' ? <LinkIcon /> : <CubeIcon />}
                        >
                          {namespace}
                        </Label>
                      </FlexItem>

                      {/* App Count */}
                      <FlexItem>
                        <span
                          className="app-dashboard__namespace-count"
                          style={{ fontWeight: 600 }}
                        >
                          {apps.length} {apps.length === 1 ? 'app' : 'apps'}
                        </span>
                      </FlexItem>

                      {/* Category Breakdown */}
                      <FlexItem>
                        <Flex spaceItems={{ default: 'spaceItemsXs' }}>
                          {Object.entries(categoryCount).map(([cat, count]) => (
                            <FlexItem key={cat}>
                              <span
                                className="app-dashboard__category-chip"
                                title={`${cat}: ${String(count)} app${count === 1 ? '' : 's'}`}
                              >
                                <span className="app-dashboard__app-category-icon">
                                  {getCategoryIcon(cat)}
                                </span>{' '}
                                {count}
                              </span>
                            </FlexItem>
                          ))}
                        </Flex>
                      </FlexItem>
                    </Flex>
                  </div>

                  {/* Swimlane Content - Apps Grid */}
                  {isExpanded && (
                    <div style={{ padding: '1rem' }}>
                      <Gallery
                        hasGutter
                        minWidths={{ default: '140px', md: '150px' }}
                        maxWidths={{ default: '180px', md: '200px' }}
                      >
                        {apps.map((app) => (
                          <Card
                            key={`${app.namespace}-${app.name}`}
                            isCompact
                            className="app-dashboard__card"
                          >
                            <CardBody
                              className="app-dashboard__card-body"
                              style={{ padding: '0.625rem' }}
                            >
                              {/* App Name */}
                              <div style={{ marginBottom: '0.375rem' }}>
                                {app.isCustomLink ? (
                                  <span
                                    className="app-dashboard__app-name"
                                    style={{
                                      fontSize: '0.875rem',
                                      display: 'block',
                                    }}
                                  >
                                    <Truncate content={app.displayName} />
                                  </span>
                                ) : (
                                  <a
                                    href={getDeploymentLink(app.namespace, app.name)}
                                    className="app-dashboard__app-link"
                                    style={{
                                      fontSize: '0.875rem',
                                      display: 'block',
                                    }}
                                  >
                                    <Truncate content={app.displayName} />
                                  </a>
                                )}
                              </div>

                              {/* Category Icon and Label */}
                              <div style={{ marginBottom: '0.375rem' }}>
                                <Flex
                                  alignItems={{ default: 'alignItemsCenter' }}
                                  spaceItems={{ default: 'spaceItemsXs' }}
                                >
                                  <FlexItem
                                    className="app-dashboard__app-category-icon"
                                    style={{ fontSize: '0.75rem' }}
                                  >
                                    {getCategoryIcon(app.category)}
                                  </FlexItem>
                                  <FlexItem>
                                    <span
                                      className="app-dashboard__category-header-count"
                                      style={{ fontSize: '0.6875rem', marginLeft: 0 }}
                                    >
                                      {app.category}
                                    </span>
                                  </FlexItem>
                                </Flex>
                              </div>

                              {/* Description (if available) */}
                              {app.description && (
                                <p
                                  className="app-dashboard__description"
                                  style={{
                                    margin: '0 0 0.375rem 0',
                                    fontSize: '0.6875rem',
                                    lineHeight: '1.3',
                                    maxHeight: '2.6em',
                                    overflow: 'hidden',
                                  }}
                                >
                                  {app.description}
                                </p>
                              )}

                              {/* Open Button */}
                              <div
                                className="app-dashboard__card-divider"
                                style={{
                                  marginTop: '0.375rem',
                                  paddingTop: '0.375rem',
                                }}
                              >
                                <Button
                                  variant="link"
                                  icon={<ExternalLinkAltIcon />}
                                  iconPosition="right"
                                  component="a"
                                  href={app.url}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  isInline
                                  style={{ padding: 0, fontSize: '0.75rem', fontWeight: 600 }}
                                >
                                  Open
                                </Button>
                              </div>
                            </CardBody>
                          </Card>
                        ))}
                      </Gallery>
                    </div>
                  )}
                </div>
              </PageSection>
            );
          })}
        </>
      );
    }

    if (viewMode === 'compact') {
      // Toggle category collapse
      const toggleCategory = (category: string) => {
        setCollapsedCategories((prev) => {
          const newSet = new Set(prev);
          if (newSet.has(category)) {
            newSet.delete(category);
          } else {
            newSet.add(category);
          }
          return newSet;
        });
      };

      return (
        <>
          {filteredCategories.map((category) => {
            const isExpanded = !collapsedCategories.has(category);
            const apps = filteredGroupedRoutes[category];

            return (
              <PageSection key={category} style={{ paddingTop: '0.5rem', paddingBottom: '0.5rem' }}>
                {/* Category Header - Clickable to expand/collapse */}
                <div
                  className="app-dashboard__category-header"
                  onClick={() => {
                    toggleCategory(category);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      toggleCategory(category);
                    }
                  }}
                  role="button"
                  tabIndex={0}
                >
                  <span className="app-dashboard__toggle-icon" style={{ marginRight: '0.5rem' }}>
                    {isExpanded ? <AngleDownIcon /> : <AngleRightIcon />}
                  </span>
                  <span
                    className="app-dashboard__category-header-icon"
                    style={{ marginRight: '0.5rem', fontSize: '1rem' }}
                  >
                    {getCategoryIcon(category)}
                  </span>
                  <Title
                    headingLevel="h3"
                    size="md"
                    style={{ marginBottom: 0, marginRight: '0.5rem' }}
                  >
                    {category}
                  </Title>
                  <span
                    className="app-dashboard__category-header-count"
                    style={{ fontSize: '0.75rem' }}
                  >
                    ({apps.length} {apps.length === 1 ? 'app' : 'apps'})
                  </span>
                </div>

                {/* Compact Table */}
                {isExpanded && (
                  <Table
                    aria-label={`${category} applications`}
                    variant="compact"
                    borders={true}
                    style={{ fontSize: '0.875rem' }}
                  >
                    <Thead>
                      <Tr>
                        <Th width={20}>Application</Th>
                        <Th width={15}>Actions</Th>
                        <Th width={15}>Namespace</Th>
                        <Th width={35}>Description</Th>
                      </Tr>
                    </Thead>
                    <Tbody>
                      {apps.map((app) => {
                        const appKey = `${app.namespace}/${app.name}`;
                        const isGroupExpanded = expandedAppGroups.has(appKey);

                        return (
                          <React.Fragment key={appKey}>
                            <Tr>
                              {/* Application Name */}
                              <Td dataLabel="Application">
                                <Flex
                                  alignItems={{ default: 'alignItemsCenter' }}
                                  spaceItems={{ default: 'spaceItemsSm' }}
                                >
                                  {app.isGrouped && (
                                    <FlexItem>
                                      <Button
                                        variant="plain"
                                        aria-label={isGroupExpanded ? 'Collapse' : 'Expand'}
                                        onClick={() => {
                                          setExpandedAppGroups((prev) => {
                                            const next = new Set(prev);
                                            if (next.has(appKey)) {
                                              next.delete(appKey);
                                            } else {
                                              next.add(appKey);
                                            }
                                            return next;
                                          });
                                        }}
                                        style={{ padding: '0.25rem', minWidth: 'auto' }}
                                      >
                                        {isGroupExpanded ? <AngleDownIcon /> : <AngleRightIcon />}
                                      </Button>
                                    </FlexItem>
                                  )}
                                  <FlexItem
                                    className="app-dashboard__table-category-icon"
                                    style={{ fontSize: '0.875rem' }}
                                  >
                                    {getCategoryIcon(app.category)}
                                  </FlexItem>
                                  <FlexItem>
                                    {app.isCustomLink ? (
                                      <span className="app-dashboard__app-name">
                                        {app.displayName}
                                      </span>
                                    ) : (
                                      <a
                                        href={getDeploymentLink(app.namespace, app.name)}
                                        className="app-dashboard__app-link"
                                      >
                                        {app.displayName}
                                      </a>
                                    )}
                                    {app.isGrouped && (
                                      <Label
                                        color="grey"
                                        isCompact
                                        style={{ marginLeft: '0.5rem' }}
                                      >
                                        {app.subDeployments?.length} deployments
                                      </Label>
                                    )}
                                  </FlexItem>
                                  <FlexItem>
                                    <Tooltip
                                      content={
                                        isFavorite(app.namespace, app.name)
                                          ? 'Remove from favorites'
                                          : 'Add to favorites'
                                      }
                                    >
                                      <Button
                                        variant="plain"
                                        aria-label={
                                          isFavorite(app.namespace, app.name)
                                            ? 'Remove from favorites'
                                            : 'Add to favorites'
                                        }
                                        onClick={(e) => {
                                          e.stopPropagation();
                                          toggleFavorite(appKey);
                                        }}
                                        style={{
                                          padding: '0.25rem',
                                          minWidth: 'auto',
                                          height: 'auto',
                                        }}
                                      >
                                        {isFavorite(app.namespace, app.name) ? (
                                          <StarIcon
                                            className="app-dashboard__favorite-icon--active"
                                            style={{ fontSize: '0.875rem' }}
                                          />
                                        ) : (
                                          <OutlinedStarIcon
                                            className="app-dashboard__favorite-icon--inactive"
                                            style={{ fontSize: '0.875rem' }}
                                          />
                                        )}
                                      </Button>
                                    </Tooltip>
                                  </FlexItem>
                                </Flex>
                              </Td>

                              {/* Actions */}
                              <Td dataLabel="Actions">
                                {app.url && (
                                  <Button
                                    variant="link"
                                    icon={<ExternalLinkAltIcon />}
                                    iconPosition="right"
                                    component="a"
                                    href={app.url}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    isInline
                                    style={{ padding: 0, fontSize: '0.8125rem' }}
                                  >
                                    Open
                                  </Button>
                                )}
                              </Td>

                              {/* Namespace */}
                              <Td dataLabel="Namespace">
                                {app.isCustomLink ? (
                                  <Label color="blue" isCompact icon={<LinkIcon />}>
                                    Custom
                                  </Label>
                                ) : (
                                  <Label
                                    color={getNamespaceColor(app.namespace)}
                                    isCompact
                                    icon={<CubeIcon />}
                                  >
                                    {app.namespace}
                                  </Label>
                                )}
                              </Td>

                              {/* Description */}
                              <Td dataLabel="Description">
                                {app.description ? (
                                  <Truncate
                                    content={app.description}
                                    tooltipPosition="top"
                                    className="app-dashboard__description-truncate"
                                    style={{ fontSize: '0.8125rem' }}
                                  />
                                ) : (
                                  <span className="app-dashboard__placeholder">No description</span>
                                )}
                              </Td>
                            </Tr>

                            {/* Sub-deployments (if grouped and expanded) */}
                            {app.isGrouped &&
                              isGroupExpanded &&
                              app.subDeployments?.map((subApp) => (
                                <Tr
                                  key={`${subApp.namespace}/${subApp.name}`}
                                  className="app-dashboard__subrow"
                                >
                                  <Td dataLabel="Application" style={{ paddingLeft: '3rem' }}>
                                    <Flex
                                      alignItems={{ default: 'alignItemsCenter' }}
                                      spaceItems={{ default: 'spaceItemsSm' }}
                                    >
                                      <FlexItem>
                                        <a
                                          href={getDeploymentLink(subApp.namespace, subApp.name)}
                                          className="app-dashboard__app-link"
                                          style={{
                                            fontSize: '0.8125rem',
                                          }}
                                        >
                                          {subApp.displayName}
                                        </a>
                                      </FlexItem>
                                    </Flex>
                                  </Td>
                                  <Td dataLabel="Actions">
                                    {subApp.url && (
                                      <Button
                                        variant="link"
                                        icon={<ExternalLinkAltIcon />}
                                        iconPosition="right"
                                        component="a"
                                        href={subApp.url}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        isInline
                                        style={{ padding: 0, fontSize: '0.8125rem' }}
                                      >
                                        Open
                                      </Button>
                                    )}
                                  </Td>
                                  <Td dataLabel="Namespace">
                                    <Label
                                      color={getNamespaceColor(subApp.namespace)}
                                      isCompact
                                      icon={<CubeIcon />}
                                    >
                                      {subApp.namespace}
                                    </Label>
                                  </Td>
                                  <Td dataLabel="Description">
                                    {subApp.description ? (
                                      <Truncate
                                        content={subApp.description}
                                        tooltipPosition="top"
                                        className="app-dashboard__description-truncate"
                                        style={{ fontSize: '0.8125rem' }}
                                      />
                                    ) : (
                                      <span className="app-dashboard__placeholder">
                                        No description
                                      </span>
                                    )}
                                  </Td>
                                </Tr>
                              ))}

                            {/* Custom Links (additional routes for sidecars/containers) */}
                            {app.customLinks?.map((link, index) => (
                              <Tr
                                key={`${app.namespace}/${app.name}/custom-link-${index.toString()}`}
                                className="app-dashboard__subrow"
                              >
                                <Td dataLabel="Application" style={{ paddingLeft: '3rem' }}>
                                  <Flex
                                    alignItems={{ default: 'alignItemsCenter' }}
                                    spaceItems={{ default: 'spaceItemsSm' }}
                                  >
                                    <FlexItem>
                                      <span
                                        className="app-dashboard__custom-link-label"
                                        style={{
                                          fontSize: '0.8125rem',
                                          fontStyle: 'italic',
                                        }}
                                      >
                                        └─ container: {link.name}
                                      </span>
                                    </FlexItem>
                                  </Flex>
                                </Td>
                                <Td dataLabel="Actions">
                                  <Button
                                    variant="link"
                                    icon={<ExternalLinkAltIcon />}
                                    iconPosition="right"
                                    component="a"
                                    href={link.url}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    isInline
                                    style={{ padding: 0, fontSize: '0.8125rem' }}
                                  >
                                    Open
                                  </Button>
                                </Td>
                                <Td dataLabel="Namespace">
                                  <Label
                                    color={getNamespaceColor(app.namespace)}
                                    isCompact
                                    icon={<CubeIcon />}
                                  >
                                    {app.namespace}
                                  </Label>
                                </Td>
                                <Td dataLabel="Description">
                                  <span
                                    className="app-dashboard__description"
                                    style={{ fontSize: '0.8125rem' }}
                                  >
                                    {link.description ?? 'Additional route'}
                                  </span>
                                </Td>
                              </Tr>
                            ))}
                          </React.Fragment>
                        );
                      })}
                    </Tbody>
                  </Table>
                )}
              </PageSection>
            );
          })}
        </>
      );
    }

    // Default: App View (current grid view)
    return (
      <>
        {filteredCategories.map((category) => (
          <PageSection key={category} style={{ paddingTop: '0.75rem', paddingBottom: '0.75rem' }}>
            <Flex alignItems={{ default: 'alignItemsCenter' }} style={{ marginBottom: '0.75rem' }}>
              <FlexItem>
                <span
                  className="app-dashboard__section-icon"
                  style={{ marginRight: '0.5rem', fontSize: '1.25rem' }}
                >
                  {getCategoryIcon(category)}
                </span>
              </FlexItem>
              <FlexItem>
                <Title headingLevel="h2" size="lg" style={{ marginBottom: 0 }}>
                  {category}
                </Title>
              </FlexItem>
              <FlexItem>
                <span className="app-dashboard__section-title-count">
                  {filteredGroupedRoutes[category].length}{' '}
                  {filteredGroupedRoutes[category].length === 1 ? 'app' : 'apps'}
                </span>
              </FlexItem>
            </Flex>
            <Gallery
              hasGutter
              minWidths={{ default: '180px', md: '200px' }}
              maxWidths={{ default: '240px', md: '260px' }}
            >
              {filteredGroupedRoutes[category].map((app) => (
                <Card
                  key={`${app.namespace}-${app.name}`}
                  isCompact
                  className="app-dashboard__card"
                >
                  <CardBody className="app-dashboard__card-body" style={{ padding: '0.75rem' }}>
                    <Split hasGutter>
                      <SplitItem isFilled>
                        <div
                          style={{
                            marginBottom: '0.4rem',
                            display: 'flex',
                            alignItems: 'center',
                            gap: '0.5rem',
                          }}
                        >
                          {app.isCustomLink ? (
                            <span
                              className="app-dashboard__app-name"
                              style={{
                                fontSize: '1rem',
                                flex: 1,
                              }}
                            >
                              {app.displayName}
                            </span>
                          ) : (
                            <a
                              href={getDeploymentLink(app.namespace, app.name)}
                              className="app-dashboard__app-link"
                              style={{
                                fontSize: '1rem',
                                flex: 1,
                              }}
                            >
                              {app.displayName}
                            </a>
                          )}
                          <Tooltip
                            content={
                              isFavorite(app.namespace, app.name)
                                ? 'Remove from favorites'
                                : 'Add to favorites'
                            }
                          >
                            <Button
                              variant="plain"
                              aria-label={
                                isFavorite(app.namespace, app.name)
                                  ? 'Remove from favorites'
                                  : 'Add to favorites'
                              }
                              onClick={(e) => {
                                e.stopPropagation();
                                toggleFavorite(`${app.namespace}/${app.name}`);
                              }}
                              style={{
                                padding: '0.25rem',
                                minWidth: 'auto',
                                height: 'auto',
                              }}
                            >
                              {isFavorite(app.namespace, app.name) ? (
                                <StarIcon
                                  className="app-dashboard__favorite-icon--active"
                                  style={{ fontSize: '1rem' }}
                                />
                              ) : (
                                <OutlinedStarIcon
                                  className="app-dashboard__favorite-icon--inactive"
                                  style={{ fontSize: '1rem' }}
                                />
                              )}
                            </Button>
                          </Tooltip>
                        </div>
                        <div style={{ marginBottom: '0.4rem' }}>
                          {app.isCustomLink ? (
                            <Label color="blue" isCompact icon={<LinkIcon />}>
                              Custom Link
                            </Label>
                          ) : (
                            <>
                              <Label
                                color={getNamespaceColor(app.namespace)}
                                isCompact
                                icon={<CubeIcon />}
                              >
                                {app.namespace}
                              </Label>
                              {app.isGrouped && (
                                <Label color="grey" isCompact style={{ marginLeft: '0.5rem' }}>
                                  {app.subDeployments?.length} deployments
                                </Label>
                              )}
                            </>
                          )}
                        </div>
                        {app.description && (
                          <p
                            className="app-dashboard__description"
                            style={{
                              margin: 0,
                              fontSize: '0.75rem',
                              lineHeight: '1.3',
                            }}
                          >
                            {app.description}
                          </p>
                        )}
                      </SplitItem>
                    </Split>
                    <div
                      className="app-dashboard__card-divider"
                      style={{
                        marginTop: '0.5rem',
                        paddingTop: '0.5rem',
                      }}
                    >
                      <Button
                        variant="link"
                        icon={<ExternalLinkAltIcon />}
                        iconPosition="right"
                        component="a"
                        href={app.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        isInline
                        style={{ padding: 0, fontSize: '0.8125rem', fontWeight: 600 }}
                      >
                        Open Application
                      </Button>
                    </div>
                  </CardBody>
                </Card>
              ))}
            </Gallery>
          </PageSection>
        ))}
      </>
    );
  };

  return (
    <div className="app-dashboard">
      <DocumentTitle>App Dashboard</DocumentTitle>
      <ListPageHeader title="App Dashboard" />

      <PageSection
        className="app-dashboard__toolbar"
        style={{
          paddingTop: '0.75rem',
          paddingBottom: '0.75rem',
        }}
      >
        <Flex
          alignItems={{ default: 'alignItemsCenter' }}
          justifyContent={{ default: 'justifyContentSpaceBetween' }}
        >
          <FlexItem>
            <Flex
              alignItems={{ default: 'alignItemsCenter' }}
              spaceItems={{ default: 'spaceItemsLg' }}
              flexWrap={{ default: 'wrap' }}
            >
              {/* View Switcher */}
              <FlexItem>
                <ToggleGroup aria-label="View mode selector">
                  <Tooltip content="Show a dense table layout for larger inventories">
                    <ToggleGroupItem
                      icon={<ThIcon />}
                      aria-label="Compact view"
                      buttonId="view-compact"
                      isSelected={viewMode === 'compact'}
                      onChange={() => {
                        setViewMode('compact');
                      }}
                    >
                      Compact
                    </ToggleGroupItem>
                  </Tooltip>
                  <Tooltip content="Group applications by namespace">
                    <ToggleGroupItem
                      icon={<StreamIcon />}
                      aria-label="Namespace view"
                      buttonId="view-namespace"
                      isSelected={viewMode === 'namespace'}
                      onChange={() => {
                        setViewMode('namespace');
                      }}
                    >
                      Namespace
                    </ToggleGroupItem>
                  </Tooltip>
                  <Tooltip content="Switch to the default app card layout">
                    <ToggleGroupItem
                      icon={<ThLargeIcon />}
                      aria-label="App view"
                      buttonId="view-app"
                      isSelected={viewMode === 'app'}
                      onChange={() => {
                        setViewMode('app');
                      }}
                    >
                      App
                    </ToggleGroupItem>
                  </Tooltip>
                </ToggleGroup>
              </FlexItem>

              <FlexItem>
                <SearchInput
                  value={searchQuery}
                  onChange={(_event, value) => {
                    setSearchQuery(value);
                  }}
                  onClear={() => {
                    setSearchQuery('');
                  }}
                  aria-label="Search applications"
                  placeholder="Search by name, namespace, or category"
                />
              </FlexItem>

              <FlexItem>
                <Label color="blue" icon={<SearchIcon />} isCompact>
                  {totalVisibleCount} of {totalAppCount} shown
                </Label>
              </FlexItem>

              {/* Description text */}
              <FlexItem>
                <p className="app-dashboard__toolbar-description">
                  Search is persisted locally • Use the view switcher to change layouts •
                  Deployments labeled or annotated with{' '}
                  <code className="app-dashboard__toolbar-code">
                    dashboard.yamlwrangler.com/enabled=true
                  </code>{' '}
                  • ConfigMaps labeled with{' '}
                  <code className="app-dashboard__toolbar-code">
                    dashboard.yamlwrangler.com/type=custom-link
                  </code>
                </p>
              </FlexItem>
            </Flex>
          </FlexItem>

          {/* Show Private toggle */}
          {hasPrivateCategory && (
            <FlexItem>
              <Flex alignItems={{ default: 'alignItemsCenter' }}>
                <FlexItem style={{ marginRight: '0.5rem' }}>
                  {showPrivate ? (
                    <EyeIcon className="app-dashboard__muted-icon" />
                  ) : (
                    <EyeSlashIcon className="app-dashboard__muted-icon" />
                  )}
                </FlexItem>
                <FlexItem>
                  <Switch
                    id="show-private-toggle"
                    label="Show Private"
                    aria-label="Toggle visibility for private applications"
                    isChecked={showPrivate}
                    onChange={(_, checked) => {
                      setShowPrivate(checked);
                    }}
                  />
                </FlexItem>
              </Flex>
            </FlexItem>
          )}
        </Flex>
      </PageSection>

      {renderContent()}
    </div>
  );
};

export default AppDashboardPage;

// Made with Bob
