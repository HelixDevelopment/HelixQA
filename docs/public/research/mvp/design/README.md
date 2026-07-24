# Helix Seller — Design System

## Overview

The Helix Seller design system provides consistent, accessible, and maintainable UI components across all platforms. It follows OpenDesign principles and supports light/dark modes with responsive design.

## OpenDesign Integration

Helix Seller integrates with OpenDesign for design-to-code workflow:

- **Design Tokens** — Exported from OpenDesign as JSON
- **Component Library** — Generated from design specifications
- **Style Guide** — Living documentation of design decisions
- **Asset Management** — Icons, illustrations, and media assets

## Design Tokens

### Color System

```json
{
  "colors": {
    "primary": {
      "50": "#eff6ff",
      "100": "#dbeafe",
      "200": "#bfdbfe",
      "300": "#93c5fd",
      "400": "#60a5fa",
      "500": "#3b82f6",
      "600": "#2563eb",
      "700": "#1d4ed8",
      "800": "#1e40af",
      "900": "#1e3a8a"
    },
    "neutral": {
      "50": "#f8fafc",
      "100": "#f1f5f9",
      "200": "#e2e8f0",
      "300": "#cbd5e1",
      "400": "#94a3b8",
      "500": "#64748b",
      "600": "#475569",
      "700": "#334155",
      "800": "#1e293b",
      "900": "#0f172a"
    },
    "success": "#22c55e",
    "warning": "#f59e0b",
    "error": "#ef4444",
    "info": "#3b82f6"
  }
}
```

### Typography

```json
{
  "typography": {
    "fontFamily": {
      "sans": "Inter, system-ui, sans-serif",
      "mono": "JetBrains Mono, monospace"
    },
    "fontSize": {
      "xs": "0.75rem",
      "sm": "0.875rem",
      "base": "1rem",
      "lg": "1.125rem",
      "xl": "1.25rem",
      "2xl": "1.5rem",
      "3xl": "1.875rem",
      "4xl": "2.25rem"
    },
    "fontWeight": {
      "normal": 400,
      "medium": 500,
      "semibold": 600,
      "bold": 700
    },
    "lineHeight": {
      "tight": 1.25,
      "normal": 1.5,
      "relaxed": 1.75
    }
  }
}
```

### Spacing

```json
{
  "spacing": {
    "0": "0",
    "1": "0.25rem",
    "2": "0.5rem",
    "3": "0.75rem",
    "4": "1rem",
    "5": "1.25rem",
    "6": "1.5rem",
    "8": "2rem",
    "10": "2.5rem",
    "12": "3rem",
    "16": "4rem",
    "20": "5rem",
    "24": "6rem"
  }
}
```

### Border Radius

```json
{
  "borderRadius": {
    "none": "0",
    "sm": "0.25rem",
    "md": "0.375rem",
    "lg": "0.5rem",
    "xl": "0.75rem",
    "2xl": "1rem",
    "full": "9999px"
  }
}
```

## Component Library

### Buttons

```html
<!-- Primary Button -->
<button class="btn btn-primary">Primary</button>

<!-- Secondary Button -->
<button class="btn btn-secondary">Secondary</button>

<!-- Outline Button -->
<button class="btn btn-outline">Outline</button>

<!-- Ghost Button -->
<button class="btn btn-ghost">Ghost</button>

<!-- Danger Button -->
<button class="btn btn-danger">Danger</button>

<!-- Sizes -->
<button class="btn btn-primary btn-sm">Small</button>
<button class="btn btn-primary btn-md">Medium</button>
<button class="btn btn-primary btn-lg">Large</button>
```

### Form Elements

```html
<!-- Input -->
<div class="form-group">
  <label class="form-label">Email</label>
  <input type="email" class="form-input" placeholder="Enter email">
  <span class="form-hint">We'll never share your email</span>
</div>

<!-- Select -->
<div class="form-group">
  <label class="form-label">Provider</label>
  <select class="form-select">
    <option>Paddle</option>
    <option>Lemon Squeezy</option>
  </select>
</div>

<!-- Checkbox -->
<label class="form-checkbox">
  <input type="checkbox">
  <span class="checkmark"></span>
  <span class="label">I agree to terms</span>
</label>

<!-- Toggle -->
<label class="form-toggle">
  <input type="checkbox">
  <span class="slider"></span>
</label>
```

### Cards

```html
<div class="card">
  <div class="card-header">
    <h3 class="card-title">Card Title</h3>
  </div>
  <div class="card-body">
    <p>Card content goes here</p>
  </div>
  <div class="card-footer">
    <button class="btn btn-primary">Action</button>
  </div>
</div>
```

### Tables

```html
<table class="table">
  <thead>
    <tr>
      <th>ID</th>
      <th>Name</th>
      <th>Status</th>
      <th>Actions</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>123</td>
      <td>Acme Corp</td>
      <td><span class="badge badge-success">Active</span></td>
      <td><button class="btn btn-sm btn-ghost">View</button></td>
    </tr>
  </tbody>
</table>
```

### Badges

```html
<span class="badge badge-success">Active</span>
<span class="badge badge-warning">Pending</span>
<span class="badge badge-error">Failed</span>
<span class="badge badge-info">Info</span>
```

### Modals

```html
<div class="modal" id="modal">
  <div class="modal-overlay"></div>
  <div class="modal-content">
    <div class="modal-header">
      <h3 class="modal-title">Modal Title</h3>
      <button class="modal-close">&times;</button>
    </div>
    <div class="modal-body">
      <p>Modal content</p>
    </div>
    <div class="modal-footer">
      <button class="btn btn-secondary">Cancel</button>
      <button class="btn btn-primary">Confirm</button>
    </div>
  </div>
</div>
```

## Light/Dark Mode

### CSS Variables

```css
:root {
  --bg-primary: #ffffff;
  --bg-secondary: #f8fafc;
  --text-primary: #0f172a;
  --text-secondary: #475569;
  --border-color: #e2e8f0;
  --shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
}

[data-theme="dark"] {
  --bg-primary: #0f172a;
  --bg-secondary: #1e293b;
  --text-primary: #f8fafc;
  --text-secondary: #94a3b8;
  --border-color: #334155;
  --shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
}
```

### Theme Toggle

```javascript
// Theme management
const theme = localStorage.getItem('theme') || 'light';
document.documentElement.setAttribute('data-theme', theme);

function toggleTheme() {
  const current = document.documentElement.getAttribute('data-theme');
  const next = current === 'light' ? 'dark' : 'light';
  document.documentElement.setAttribute('data-theme', next);
  localStorage.setItem('theme', next);
}
```

## Responsive Design

### Breakpoints

```json
{
  "breakpoints": {
    "sm": "640px",
    "md": "768px",
    "lg": "1024px",
    "xl": "1280px",
    "2xl": "1536px"
  }
}
```

### Responsive Utilities

```html
<!-- Show on mobile only -->
<div class="block sm:hidden">Mobile only</div>

<!-- Show on desktop only -->
<div class="hidden sm:block">Desktop only</div>

<!-- Responsive grid -->
<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
  <div class="card">1</div>
  <div class="card">2</div>
  <div class="card">3</div>
</div>
```

## Accessibility (WCAG 2.1 AA)

### Requirements

- **Color Contrast** — Minimum 4.5:1 for normal text, 3:1 for large text
- **Keyboard Navigation** — All interactive elements focusable and operable
- **Screen Reader** — ARIA labels for all interactive elements
- **Focus Indicators** — Visible focus states for keyboard users
- **Alt Text** — Descriptive alt text for all images
- **Form Labels** — All form inputs have associated labels
- **Error Messages** — Clear, accessible error messages

### Implementation

```html
<!-- Accessible button -->
<button 
  class="btn btn-primary"
  aria-label="Create new merchant"
  role="button"
>
  Create Merchant
</button>

<!-- Accessible form -->
<div class="form-group">
  <label for="email" class="form-label">Email address</label>
  <input 
    type="email" 
    id="email" 
    class="form-input"
    aria-describedby="email-help"
    aria-required="true"
  >
  <span id="email-help" class="form-hint">Required for account login</span>
</div>

<!-- Accessible table -->
<table aria-label="Merchant list">
  <thead>
    <tr>
      <th scope="col">Name</th>
      <th scope="col">Status</th>
    </tr>
  </thead>
</table>
```

### Testing

```bash
# Install axe CLI
npm install -g @axe-core/cli

# Test URL
axe https://app.helix.dev

# Or use browser extension
# axe DevTools for Chrome/Firefox
```

## Documentation

- **Storybook** — Interactive component documentation
- **Design Tokens** — Exported from OpenDesign
- **Style Guide** — Living documentation at `/design`
- **Component API** — Auto-generated from code comments

## Contributing

1. Follow existing component patterns
2. Add Storybook stories for new components
3. Ensure accessibility compliance
4. Test in both light and dark modes
5. Test responsive behavior
6. Document component API and usage
