const nextJest = require('next/jest')

const createJestConfig = nextJest({
  // Points to the Next.js app root so jest can load next.config.js and .env files
  dir: './',
})

/** @type {import('jest').Config} */
const config = {
  coverageProvider: 'v8',
  testEnvironment: 'jsdom',
  setupFilesAfterEnv: ['<rootDir>/jest.setup.ts'],
  // Allow absolute imports via @/* path alias
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/$1',
  },
  // Initial threshold set low (10%) for skeleton code; raise as real logic is added.
  coverageThreshold: {
    global: {
      lines: 10,
      functions: 10,
      branches: 0,
      statements: 10,
    },
  },
}

module.exports = createJestConfig(config)
