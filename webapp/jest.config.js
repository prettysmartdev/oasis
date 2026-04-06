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
  // Raised to 50% line/function/statement and 40% branch coverage for work item 0004.
  coverageThreshold: {
    global: {
      lines: 50,
      functions: 50,
      branches: 40,
      statements: 50,
    },
  },
}

module.exports = createJestConfig(config)
