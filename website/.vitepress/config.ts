import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'HelixQA',
  description: 'Autonomous QA Robot — Fire-and-Forget Quality Assurance',
  themeConfig: {
    logo: '/logo.svg',
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Quick Start', link: '/quick-start' },
      { text: 'User Manual', link: '/manual/' },
      { text: 'Architecture', link: '/architecture' },
      { text: 'Video Course', link: '/course' },
      { text: 'GitHub', link: 'https://github.com/HelixDevelopment/HelixQA' }
    ],
    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Introduction', link: '/introduction' },
          { text: 'Quick Start', link: '/quick-start' },
          { text: 'Installation', link: '/installation' },
        ]
      },
      {
        text: 'Core Concepts',
        items: [
          { text: 'Architecture', link: '/architecture' },
          { text: 'Pipeline Phases', link: '/pipeline' },
          { text: 'LLM Providers', link: '/providers' },
          { text: 'Platform Executors', link: '/executors' },
        ]
      },
      {
        text: 'User Manual',
        items: [
          { text: 'CLI Reference', link: '/manual/cli' },
          { text: 'Configuration', link: '/manual/config' },
          { text: 'Memory System', link: '/manual/memory' },
          { text: 'Issue Tickets', link: '/manual/tickets' },
          { text: 'Multi-Pass QA', link: '/manual/multi-pass' },
        ]
      },
      {
        text: 'Advanced',
        items: [
          { text: 'Containerization', link: '/advanced/containers' },
          { text: 'Open-Source Tools', link: '/advanced/tools' },
          { text: 'Challenges', link: '/advanced/challenges' },
          { text: 'Video Course', link: '/course' },
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/HelixDevelopment/HelixQA' }
    ],
    footer: {
      message: 'Built by Vasic Digital',
      copyright: 'Copyright 2024-2026 Vasic Digital'
    }
  }
})
