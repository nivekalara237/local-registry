import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';
import githubAlerts from './src/remark/github-alerts.mjs';

// Where the site is served; override for an internal host:
// DOCS_URL=https://docs.example.com DOCS_BASE_URL=/ npm run build
const url = process.env.DOCS_URL ?? 'https://nivekalara237.github.io';
const baseUrl = process.env.DOCS_BASE_URL ?? '/';
const repo = 'https://github.com/nivekalara237/local-registry';

const config: Config = {
  title: 'Local Registry',
  tagline: 'A private release registry for builds and any versioned files',
  favicon: 'img/favicon.ico',
  future: {v4: true},
  url,
  baseUrl,
  organizationName: 'nivekalara237',
  projectName: 'local-registry',
  trailingSlash: false,

  onBrokenLinks: 'throw',
  onBrokenAnchors: 'throw',
  markdown: {
    mermaid: true,
    hooks: {onBrokenMarkdownLinks: 'throw'},
  },
  themes: [
    '@docusaurus/theme-mermaid',
    [
      '@easyops-cn/docusaurus-search-local',
      {hashed: true, language: ['en', 'fr'], indexBlog: false, docsRouteBasePath: '/docs'},
    ],
  ],

  i18n: {
    defaultLocale: 'en',
    locales: ['en', 'fr'],
    localeConfigs: {en: {label: 'English'}, fr: {label: 'Français'}},
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: `${repo}/edit/main/website/`,
          editLocalizedFiles: true,
          showLastUpdateTime: true,
          beforeDefaultRemarkPlugins: [githubAlerts],
        },
        blog: false,
        theme: {customCss: './src/css/custom.css'},
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    colorMode: {respectPrefersColorScheme: true},
    navbar: {
      title: 'Local Registry',
      logo: {alt: 'local-registry', src: 'img/logo.svg'},
      items: [
        {type: 'docSidebar', sidebarId: 'docs', position: 'left', label: 'Documentation'},
        {to: '/docs/reference/api', label: 'API', position: 'left'},
        {type: 'localeDropdown', position: 'right'},
        {href: repo, label: 'GitHub', position: 'right'},
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Start',
          items: [
            {label: 'Quick start', to: '/docs/getting-started/quick-start'},
            {label: 'Concepts', to: '/docs/concepts/products-and-versions'},
          ],
        },
        {
          title: 'Reference',
          items: [
            {label: 'HTTP API', to: '/docs/reference/api'},
            {label: 'Command line', to: '/docs/reference/cli'},
            {label: 'Errors', to: '/docs/reference/errors'},
          ],
        },
        {
          title: 'Project',
          items: [
            {label: 'GitHub', href: repo},
            {label: 'Changelog', to: '/docs/changelog'},
          ],
        },
      ],
      copyright: `Local Registry, Apache License 2.0. Documentation built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'yaml', 'json', 'go', 'ini'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
