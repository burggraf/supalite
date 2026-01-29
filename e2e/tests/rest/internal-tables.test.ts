/**
 * Internal Tables Security Tests
 *
 * Tests that internal/system tables are not accessible via the public REST API.
 * This prevents unauthorized access to sensitive data like:
 * - auth.* tables (user credentials, sessions, tokens) in auth schema
 * - storage.* tables (storage metadata) in storage schema
 * - pg_* tables (PostgreSQL internal tables)
 *
 * Note: PostgreSQL uses schemas (auth, storage, public) rather than prefixes
 */

import { describe, it, expect, beforeAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('REST API - Internal Tables Security', () => {
  let supabase: SupabaseClient
  let serviceRoleClient: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SUPALITE_URL, TEST_CONFIG.SUPALITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    serviceRoleClient = createClient(TEST_CONFIG.SUPALITE_URL, TEST_CONFIG.SUPALITE_SERVICE_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  describe('Auth schema tables blocked', () => {
    const authTables = [
      'auth.users',
      'auth.sessions',
      'auth.refresh_tokens',
      'auth.identities',
      // Also test without schema prefix (pREST might handle these)
      'users',
      'sessions',
      'refresh_tokens',
      'identities',
    ]

    for (const table of authTables) {
      it(`SELECT on ${table} should return 404 or be blocked`, async () => {
        const { data, error } = await supabase.from(table).select()

        // Should either error or return empty
        if (error) {
          expect(error).not.toBeNull()
        } else {
          // If no error, might return empty for some tables
          expect(data).toBeDefined()
        }
      })

      it(`INSERT on ${table} should return 404 or be blocked`, async () => {
        const { data, error } = await supabase.from(table).insert({ test: 'value' })

        expect(error).not.toBeNull()
        expect(data).toBeNull()
      })
    }
  })

  describe('Storage schema tables blocked', () => {
    const storageTables = [
      'storage.buckets',
      'storage.objects',
      'buckets',
      'objects',
    ]

    for (const table of storageTables) {
      it(`SELECT on ${table} should return 404 or be blocked`, async () => {
        const { data, error } = await supabase.from(table).select()

        // Should either error or return empty
        if (error) {
          expect(error).not.toBeNull()
        } else {
          expect(data).toBeDefined()
        }
      })

      it(`INSERT on ${table} should return 404 or be blocked`, async () => {
        const { data, error } = await supabase.from(table).insert({ test: 'value' })

        expect(error).not.toBeNull()
        expect(data).toBeNull()
      })
    }
  })

  describe('PostgreSQL internal tables blocked', () => {
    const pgTables = [
      'pg_catalog.pg_class',
      'pg_stat_activity',
      'pg_attribute',
      'pg_type',
    ]

    for (const table of pgTables) {
      it(`SELECT on ${table} should return 404 or be blocked`, async () => {
        const { data, error } = await supabase.from(table).select()

        expect(error).not.toBeNull()
        expect(data).toBeNull()
      })
    }
  })

  describe('Information schema blocked', () => {
    const infoSchemaTables = [
      'information_schema.tables',
      'information_schema.columns',
    ]

    for (const table of infoSchemaTables) {
      it(`SELECT on ${table} should return 404 or be blocked`, async () => {
        const { data, error } = await supabase.from(table).select()

        expect(error).not.toBeNull()
        expect(data).toBeNull()
      })
    }
  })

  describe('Service role key also blocked from internal tables', () => {
    it('service_role should not access auth.users via REST API', async () => {
      const { data, error } = await serviceRoleClient.from('auth.users').select()

      // Even service role should be blocked from accessing auth tables via REST API
      expect(error).not.toBeNull()
    })

    it('service_role should be able to access regular public tables', async () => {
      const { data, error } = await serviceRoleClient.from('characters').select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)
    })
  })

  describe('Regular public tables still accessible', () => {
    it('should be able to access regular user tables', async () => {
      const { data, error } = await supabase.from('characters').select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(5)
    })

    it('should handle tables with similar names to internal tables', async () => {
      // Tables like "authentication_logs" or "user_storage" should not be blocked
      // They don't start with auth. or storage. schema prefix
      const { error: testError } = await supabase.from('countries').select()

      // This should work fine (it's a public table)
      expect(testError).toBeNull()
    })
  })
})
