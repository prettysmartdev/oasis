/** @type {import('tailwindcss').Config} */
const { fontFamily } = require('tailwindcss/defaultTheme')

module.exports = {
  darkMode: ['class'],
  content: [
    './app/**/*.{ts,tsx}',
    './components/**/*.{ts,tsx}',
  ],
  safelist: [
    'bg-green-400',
    'bg-amber-400',
    'bg-gray-400',
    'animate-pulse',
  ],
  theme: {
    extend: {
      colors: {
        // OaSis brand colours
        primary: '#2DD4BF',   // muted teal-green
        accent: '#F59E0B',    // warm amber
      },
      fontFamily: {
        sans: ['var(--font-geist-sans)', ...fontFamily.sans],
        mono: ['var(--font-geist-mono)', ...fontFamily.mono],
      },
    },
  },
  plugins: [],
}
