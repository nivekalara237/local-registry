import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    'intro',
    {
      type: 'category',
      label: 'Getting started',
      collapsed: false,
      items: ['getting-started/quick-start', 'getting-started/first-release-goreleaser', 'getting-started/first-release-files'],
    },
    {
      type: 'category',
      label: 'Concepts',
      items: [
        'concepts/products-and-versions',
        'concepts/staging-and-publish',
        'concepts/latest',
        'concepts/standard-v1',
        'concepts/integrity',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: [
        'guides/publish-with-goreleaser',
        'guides/publish-plain-files',
        'guides/ci-pipelines',
        'guides/write-registry-yaml',
        'guides/write-changelog',
        'guides/consume-releases',
        'guides/change-latest',
        'guides/windows',
      ],
    },
    {
      type: 'category',
      label: 'Operations',
      items: [
        'operations/install',
        'operations/configuration',
        'operations/security',
        'operations/quotas-and-staging',
        'operations/audit-log',
        'operations/storage-and-backup',
        'operations/monitoring',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        {
          type: 'category',
          label: 'HTTP API',
          link: {type: 'doc', id: 'reference/api/index'},
          items: [
            'reference/api/products',
            'reference/api/artifacts',
            'reference/api/downloads',
            'reference/api/upload',
            'reference/api/legacy',
          ],
        },
        'reference/cli',
        'reference/errors',
        'reference/registry-yaml',
        'reference/release-json',
        'reference/index-json',
        'reference/changelog-format',
        'reference/upload-rules',
        'reference/scripts',
        'reference/go-library',
      ],
    },
    'troubleshooting',
    'limitations',
    'glossary',
    'changelog',
  ],
};

export default sidebars;
