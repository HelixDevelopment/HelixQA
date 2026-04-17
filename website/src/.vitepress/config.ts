import { defineConfig } from 'vitepress';

export default defineConfig({
  title: 'HelixQA',
  description: 'Autonomous cross-platform QA platform.',
  lang: 'en-US',
  cleanUrls: true,
  themeConfig: {
    nav: [
      { text: 'Nexus', link: '/nexus/' },
      { text: 'Getting started', link: '/nexus/getting-started' },
      { text: 'Architecture', link: '/nexus/architecture' },
    ],
    sidebar: {
      '/nexus/': [
        {
          text: 'Helix Nexus',
          items: [
            { text: 'Overview', link: '/nexus/' },
            { text: 'Getting started', link: '/nexus/getting-started' },
            { text: 'Architecture', link: '/nexus/architecture' },
            { text: 'Video course', link: '/nexus/video-course' },
          ],
        },
      ],
    },
    footer: {
      copyright: '© HelixQA — Helix Nexus shipped under Apache 2.0.',
    },
  },
});
