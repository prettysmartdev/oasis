import { fetchApps, fetchStatus, ApiError } from '@/lib/api'

describe('api helpers', () => {
  beforeEach(() => {
    jest.resetAllMocks()
  })

  describe('fetchApps', () => {
    it('returns typed App[] on 200', async () => {
      const mockData = {
        items: [
          {
            id: 'abc123',
            name: 'my-app',
            slug: 'my-app',
            upstreamURL: 'http://localhost:8080',
            displayName: 'My App',
            description: 'A cool app',
            icon: '🚀',
            tags: ['app'],
            enabled: true,
            health: 'healthy' as const,
            createdAt: '2024-01-01T00:00:00Z',
            updatedAt: '2024-01-01T00:00:00Z',
          },
        ],
        total: 1,
      }

      global.fetch = jest.fn().mockResolvedValue({
        ok: true,
        json: async () => mockData,
      } as Response)

      const result = await fetchApps()

      expect(result.items).toHaveLength(1)
      expect(result.items[0].id).toBe('abc123')
      expect(result.items[0].health).toBe('healthy')
      expect(result.total).toBe(1)
      expect(global.fetch).toHaveBeenCalledWith('/api/v1/apps')
    })

    it('throws ApiError with correct status on non-200 response', async () => {
      global.fetch = jest.fn().mockResolvedValue({
        ok: false,
        status: 404,
        statusText: 'Not Found',
        json: async () => ({ error: 'resource not found' }),
      } as Response)

      await expect(fetchApps()).rejects.toThrow(ApiError)
      await expect(fetchApps()).rejects.toMatchObject({
        status: 404,
        message: 'resource not found',
      })
    })

    it('throws ApiError using statusText when error body has no error field', async () => {
      global.fetch = jest.fn().mockResolvedValue({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
        json: async () => ({}),
      } as Response)

      await expect(fetchApps()).rejects.toThrow(ApiError)
      await expect(fetchApps()).rejects.toMatchObject({
        status: 500,
        message: 'Internal Server Error',
      })
    })

    it('propagates network errors gracefully', async () => {
      global.fetch = jest.fn().mockRejectedValue(new TypeError('Failed to fetch'))

      await expect(fetchApps()).rejects.toThrow('Failed to fetch')
    })

    it('throws ApiError on 401 unauthorized', async () => {
      global.fetch = jest.fn().mockResolvedValue({
        ok: false,
        status: 401,
        statusText: 'Unauthorized',
        json: async () => ({ error: 'unauthorized' }),
      } as Response)

      let err: ApiError | null = null
      try {
        await fetchApps()
      } catch (e) {
        err = e as ApiError
      }
      expect(err).toBeInstanceOf(ApiError)
      expect(err?.status).toBe(401)
      expect(err?.name).toBe('ApiError')
    })
  })

  describe('fetchStatus', () => {
    it('returns Status on 200', async () => {
      const mockStatus = {
        tailscaleConnected: true,
        tailscaleIP: '100.64.0.1',
        tailscaleHostname: 'oasis',
        nginxStatus: 'running',
        registeredAppCount: 5,
        version: '1.0.0',
      }

      global.fetch = jest.fn().mockResolvedValue({
        ok: true,
        json: async () => mockStatus,
      } as Response)

      const result = await fetchStatus()

      expect(result.tailscaleConnected).toBe(true)
      expect(result.nginxStatus).toBe('running')
      expect(result.registeredAppCount).toBe(5)
      expect(global.fetch).toHaveBeenCalledWith('/api/v1/status')
    })

    it('throws ApiError on non-200 response', async () => {
      global.fetch = jest.fn().mockResolvedValue({
        ok: false,
        status: 503,
        statusText: 'Service Unavailable',
        json: async () => ({ error: 'controller offline' }),
      } as Response)

      await expect(fetchStatus()).rejects.toThrow(ApiError)
    })

    it('handles network failure gracefully', async () => {
      global.fetch = jest.fn().mockRejectedValue(new TypeError('Network request failed'))

      await expect(fetchStatus()).rejects.toThrow('Network request failed')
    })
  })

  describe('ApiError', () => {
    it('has correct name, status, and message', () => {
      const err = new ApiError(422, 'validation failed')
      expect(err.name).toBe('ApiError')
      expect(err.status).toBe(422)
      expect(err.message).toBe('validation failed')
      expect(err).toBeInstanceOf(Error)
    })
  })
})
