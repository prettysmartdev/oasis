# User Interface

## Style

Aesthetic:
- Clean, minimal phone/tablet homescreen aesthetic — a calm, organized homescreen amid the chaos of vibe-coding
- Background is a subtle color gradient generated dynamically based on the time of day (orange/red for sunrise/sunset, blue/white for midday, black/silver for night)
- Dark mode UI by default with light mode support; respects the user's system color-scheme preference
- Generous icons; 3x4 icon-based layout a la iOS where each app is a distinct, modern icon
- Subtle, purposeful animations (icon hover lift, status indicator pulse, background slowly moves and morphs); nothing distracting

Brand and colors:
- Name: OaSis
- Primary: muted teal-green (#2DD4BF range) — evokes calm, nature, a place of refuge
- Accent: warm amber (#F59E0B range) — used for interactive CTAs, "Open" buttons, focus rings
- Neutrals: cool gray scale for borders, and secondary text (Tailwind slate palette)
- Font: Geist Sans for UI text, Geist Mono for code snippets and technical values (e.g. upstream URLs)
- Logo: stylized palm tree; simple enough to render at 16px favicon size

Desktop vs mobile:
- Mobile-first design; the primary use case is opening the homescreen on a phone or tablet to navigate to an app
- Fully responsive: icon grid collapses from 6 columns (1280px+) to 4 (1024px) to 3 (mobile)
- Touch targets sized for mobile use (minimum 44px); the homescreen should be optimized for a phone on the tailnet but usable on desktop

## Usage

Layout:
- Bottom navigation: OaSis logo floating bottom left (not too big, modern floating, shimmering vibe), settings icon (with "general system status" pulsing status indicator) bottom right. 
	- Tapping OaSis logo causes it to slide over to the right, with a messaging app-style chat box trailing it. Full-width at bottom of screen on mobile, stays docked bottom-left on tablet/desktop. Tapping chat box opens a chatbot-style conversation over top of the icon grid (shimmering background, chat bubbles).
- Main content area: responsive icon grid of registered apps, optionally grouped by tags
- Each app icon contains: app icon (emoji or image, rounded corners), display name (bold, below icon), health status indicator light (Healthy / Unreachable).

Menus:
- Minimal navigation; almost all management is done via the CLI, not the dashboard
- Settings icon in the nav opens to a read-only settings card (hostname, version, registered app count, health of components) — configuration requires the CLI

Empty states:
- First launch (no apps registered): centered illustration of a desert with a single palm tree, headline "Your OaSis awaits", subline "Add your first app from the terminal", and a code block showing the `OaSis app add` command
- No apps matching active tag filter: "No apps tagged [tag]" with a prompt to clear the filter
- App icon for an unreachable upstream: card is slightly desaturated, health indicator pulsing amber, tapping opens a card explaining the error (controller should provide basic details like HTTP error code or other.)

Accessibility:
- Semantic HTML throughout; proper heading hierarchy (h1 for page title, h2 for section headings, h3 for card names)
- All interactive elements are keyboard-navigable with visible focus rings (using the accent amber color)
- ARIA labels on icon-only buttons (settings icon, OaSis icon) and status badges
- Minimum WCAG AA contrast ratios for all text and interactive elements
- Respect prefers-reduced-motion: disable card hover animations and status pulse for users who have enabled reduced motion

Machine use:
- The controller's management API is the primary machine interface; use it for scripting, automation, and integrations
- The webapp does not expose a machine-readable interface; all programmatic access should use the management API
- The OaSis CLI wraps the management API for convenient scripting from the host machine
