import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://giovi321.github.io',
  base: '/rmfakecloud',
  integrations: [
    starlight({
      title: 'rmfakecloud',
      description: 'Self hosted reMarkable sync cloud, giovi321 fork with working v6 export',
      social: [
        { icon: 'github', label: 'GitHub', href: 'https://github.com/giovi321/rmfakecloud' },
      ],
      editLink: {
        baseUrl: 'https://github.com/giovi321/rmfakecloud/edit/master/docs/',
      },
      sidebar: [
        { label: 'Home', link: '/' },
        {
          label: 'This fork',
          items: [
            { label: 'What it changes', link: '/fork/' },
            { label: 'Branches and upstream PRs', link: '/fork/branches/' },
            { label: 'Building this fork', link: '/fork/building/' },
            { label: 'How the placement was measured', link: '/fork/measurement/' },
          ],
        },
        {
          label: 'Cloud setup',
          items: [
            { label: 'From source', link: '/install/source/' },
            { label: 'With Docker', link: '/install/docker/' },
            { label: 'With Helm', link: '/install/helm/' },
            { label: 'Configuration', link: '/install/configuration/' },
            { label: 'OIDC with Authentik', link: '/install/oidc/authentik/' },
            { label: 'OIDC with Authelia', link: '/install/oidc/authelia/' },
            { label: 'Apache', link: '/install/reverse-proxy/apache/' },
            { label: 'Nginx', link: '/install/reverse-proxy/nginx/' },
            { label: 'Traefik', link: '/install/reverse-proxy/traefik/' },
            { label: 'Fail2ban', link: '/install/fail2ban/' },
            { label: 'External access', link: '/install/external-access/' },
          ],
        },
        {
          label: 'Device setup',
          items: [
            { label: 'Device', link: '/remarkable/setup/' },
            { label: 'HTTPS', link: '/remarkable/https/' },
            { label: 'Desktop client', link: '/remarkable/desktop-client/' },
          ],
        },
        {
          label: 'Usage',
          items: [
            { label: 'User profile', link: '/usage/userprofile/' },
            { label: 'Integrations', link: '/usage/integrations/' },
            { label: 'Diff sync', link: '/usage/diff-sync/' },
            { label: 'Passcode reset', link: '/usage/passcode-reset/' },
          ],
        },
        { label: 'Browser extension', link: '/browser-extension/' },
      ],
    }),
  ],
});
