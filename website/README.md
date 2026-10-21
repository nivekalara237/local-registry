# local-registry documentation site

The documentation of local-registry, built with [Docusaurus](https://docusaurus.io).
Pages are plain Markdown, readable on GitHub too:

- `docs/`: English (source)
- `i18n/fr/docusaurus-plugin-content-docs/current/`: French translation, same paths
- `sidebars.ts`: navigation; `src/pages/index.tsx`: home page

```sh
npm ci
npm start                 # http://localhost:3000, English
npm run start:fr          # French (dev serves one locale at a time)
npm run build             # fails on any broken link or anchor
npm run serve             # serves build/, both languages
```

Callouts use the GitHub syntax (`> [!NOTE]`, `> [!WARNING]`…), converted for the site by
`src/remark/github-alerts.mjs`. Diagrams are `mermaid` code blocks.

The site URL defaults to GitHub Pages; for an internal host:
`DOCS_URL=https://docs.example.com DOCS_BASE_URL=/ npm run build`.
