import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import Translate from '@docusaurus/Translate';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';

import styles from './index.module.css';

type Path = {title: ReactNode; body: ReactNode; to: string; cta: ReactNode};

const paths: Path[] = [
  {
    title: <Translate id="home.publish.title">I publish releases</Translate>,
    body: (
      <Translate id="home.publish.body">
        Send a goreleaser build, or any set of files, from your laptop or your CI. Every version is checked and
        published atomically.
      </Translate>
    ),
    to: '/docs/guides/publish-with-goreleaser',
    cta: <Translate id="home.publish.cta">Publishing guides</Translate>,
  },
  {
    title: <Translate id="home.operate.title">I run the registry</Translate>,
    body: (
      <Translate id="home.operate.body">
        One static binary, no database. Install it, secure it with a token and HTTPS, set quotas, read the audit
        log.
      </Translate>
    ),
    to: '/docs/operations/install',
    cta: <Translate id="home.operate.cta">Operations</Translate>,
  },
  {
    title: <Translate id="home.consume.title">I consume releases</Translate>,
    body: (
      <Translate id="home.consume.body">
        Find the latest version, its files and their checksums through a stable JSON API: release.json and
        index.json.
      </Translate>
    ),
    to: '/docs/guides/consume-releases',
    cta: <Translate id="home.consume.cta">Consume releases</Translate>,
  },
];

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout title={siteConfig.title} description={siteConfig.tagline}>
      <header className={styles.hero}>
        <div className="container">
          <Heading as="h1">{siteConfig.title}</Heading>
          <p className={styles.tagline}>
            <Translate id="home.tagline">
              A private release registry for goreleaser builds and any versioned files.
            </Translate>
          </p>
          <div className={styles.buttons}>
            <Link className="button button--primary button--lg" to="/docs/getting-started/quick-start">
              <Translate id="home.quickstart">Quick start, 5 minutes</Translate>
            </Link>
            <Link className="button button--secondary button--lg" to="/docs/intro">
              <Translate id="home.what">What is local-registry?</Translate>
            </Link>
          </div>
        </div>
      </header>
      <main className="container margin-vert--xl">
        <div className="row">
          {paths.map((p, i) => (
            <div key={i} className="col col--4 margin-bottom--lg">
              <div className={styles.card}>
                <Heading as="h2">{p.title}</Heading>
                <p>{p.body}</p>
                <Link to={p.to}>{p.cta} →</Link>
              </div>
            </div>
          ))}
        </div>
      </main>
    </Layout>
  );
}
