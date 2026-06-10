# Public Assets

This directory contains static assets that are copied to the root of the distribution during the build process.

## Files

- **icon.svg** - The app dashboard icon used in the OpenShift console application menu. This SVG is served directly at `https://app-dashboard.apps.sno.yamlwrangler.com/icon.svg` and referenced in the ConsoleLink resource.

## Build Process

The webpack configuration (see `webpack.config.ts`) uses `CopyWebpackPlugin` to copy all files from this directory to the root of the `dist` folder during build. These files are then included in the Docker image and served by nginx.

## Usage

To add new static assets:
1. Place the file in this directory
2. Rebuild the project with `yarn build`
3. The file will be available at `https://app-dashboard.apps.sno.yamlwrangler.com/<filename>`