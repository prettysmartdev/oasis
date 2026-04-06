# User Interface

## Style

Aesthetic:
- Clean, minimal phone/tablet homescreen aesthetic — a calm, organized homescreen amid the chaos of vibe-coding
	- The homescreen has two pages. The first page is labeled "AGENTS". The second page is labeled "APPS". Both titles are always visible at the top of the homescreen above the icons, AGENTS on the left and APPS on the right.
	- Each page vertically scrolls freely as more icons get added. If the user scrolls horizontally between the two pages, the scrolling snaps to each page (it's not freeform)
	- As the user moves horizontally between the two pages, the two titles change their size and opacity to indicate which page the user is on. The two titles should fluidly animate as the user moves the page left and right, with the "incoming" page's title becoming slightly larger and more opaque, while the "outgoing" page's title becomes smaller and more translucent.
- Background is a subtle color gradient generated dynamically based on the time of day (orange/red for sunrise/sunset, blue/white for midday, black/silver for night)
- Dark mode UI by default with light mode support; respects the user's system color-scheme preference
- Generous icons; 3x4 rounded-rect icon-based layout a la iOS where each app is a distinct, modern icon which can be an image or an emoji with a solid-colored background.
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
- Bottom navigation: 
	- OaSis logo (the palm tree emoji, within a circle) floating bottom left (not too big, modern floating, irridecent shimmering vibe, reacts to phone tilt to change shimmer interactively), 
	- Settings icon (gear emoji, within a circle) with "system status" pulsing status indicator attached bottom right. 
	- Tapping OaSis logo causes it to slide over to the right, with a messaging app-style chat box trailing it. Full-width at bottom of screen on mobile, stays docked bottom-left on tablet/desktop. Tapping chat box opens a chatbot-style conversation over top of the icon grid (shimmering background, chat bubbles).
- Main content area: responsive icon grid of registered agents/apps, optionally grouped by tags
- Each agent/app icon: app icon (emoji or image, rounded corners), display name (bold, below icon), health status indicator light (Healthy / Unreachable) to the right of the label.

Menus:
- Minimal navigation; almost all management is done via the CLI, not the dashboard
- Settings icon in the nav opens to a read-only settings card (hostname, version, registered app count, health of components) — configuration requires the CLI

Empty states:
- First launch (no agents/apps registered): 
	- agent page: centered illustration of a desert with a robot standing under a single palm tree, headline "Your OaSis awaits".
	- apps page centered illustration of a lake with a small log cabin, headline "Your OaSis awaits"
	- subline on both pages: "Add your first agent/app from the terminal", and a code block showing the `oasis app add` CLI command.
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
