/**
 * UPDATE Operation Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/update
 *
 * Each test corresponds to an example from the documentation.
 */

import { describe, it, expect, beforeAll, beforeEach, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('REST API - UPDATE Operations', () => {
  let supabase: SupabaseClient
  const testInstrumentId = 9001

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SUPALITE_URL, TEST_CONFIG.SUPALITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  beforeEach(async () => {
    await supabase
      .from('instruments')
      .insert({ id: testInstrumentId, name: 'test_instrument', section_id: 1 })
  })

  afterEach(async () => {
    await supabase.from('instruments').delete().eq('id', testInstrumentId)
  })

  /**
   * Example 1: Updating your data
   * Docs: https://supabase.com/docs/reference/javascript/update#updating-your-data
   */
  describe('1. Updating your data', () => {
    it('should update a record matching the filter', async () => {
      const { error } = await supabase
        .from('instruments')
        .update({ name: 'updated_piano' })
        .eq('id', testInstrumentId)

      expect(error).toBeNull()

      const { data: verify } = await supabase
        .from('instruments')
        .select()
        .eq('id', testInstrumentId)

      expect(verify![0].name).toBe('updated_piano')
    })

    it('should update multiple fields at once', async () => {
      const { error } = await supabase
        .from('instruments')
        .update({ name: 'updated_harp', section_id: 2 })
        .eq('id', testInstrumentId)

      expect(error).toBeNull()

      const { data: verify } = await supabase
        .from('instruments')
        .select()
        .eq('id', testInstrumentId)

      expect(verify![0].name).toBe('updated_harp')
      expect(verify![0].section_id).toBe(2)
    })
  })

  /**
   * Example 2: Update a record and return it
   * Docs: https://supabase.com/docs/reference/javascript/update#update-a-record-and-return-it
   */
  describe('2. Update a record and return it', () => {
    it('should update and return the updated record', async () => {
      const { data, error } = await supabase
        .from('instruments')
        .update({ name: 'returned_piano' })
        .eq('id', testInstrumentId)
        .select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0].name).toBe('returned_piano')
    })

    it('should return only selected columns', async () => {
      const { data, error } = await supabase
        .from('instruments')
        .update({ name: 'partial_return' })
        .eq('id', testInstrumentId)
        .select('name')

      expect(error).toBeNull()
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).not.toHaveProperty('id')
    })
  })

  /**
   * Example 3: Updating JSON data
   * Docs: https://supabase.com/docs/reference/javascript/update#updating-json-data
   */
  describe('3. Updating JSON data', () => {
    it('should update nested JSON fields (PostgreSQL JSONB)', async () => {
      // PostgreSQL supports native JSONB with arrow operators
      const { data, error } = await supabase
        .from('users')
        .update({
          address: { street: 'Melrose Place', postcode: 90210 },
        })
        .eq('address->>postcode', '10001')
        .select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
    })

    it('should update JSONB stored as native PostgreSQL type', async () => {
      await supabase.from('users').upsert({
        id: 9001,
        name: 'Test User',
        address: { street: '123 Test St', city: 'Test City', postcode: 12345 },
      })

      const { data, error } = await supabase
        .from('users')
        .update({
          address: { street: 'Updated St', city: 'New City', postcode: 54321 },
        })
        .eq('id', 9001)
        .select()

      expect(error).toBeNull()

      await supabase.from('users').delete().eq('id', 9001)
    })
  })

  describe('Additional UPDATE functionality', () => {
    it('should not update any records if filter matches none', async () => {
      const { data, error } = await supabase
        .from('instruments')
        .update({ name: 'no_match' })
        .eq('id', -9999)
        .select()

      expect(error).toBeNull()
      expect(data).toEqual([])
    })

    it('should update multiple records matching filter', async () => {
      await supabase.from('instruments').insert([
        { id: 9002, name: 'batch_test', section_id: 1 },
        { id: 9003, name: 'batch_test', section_id: 1 },
      ])

      const { data, error } = await supabase
        .from('instruments')
        .update({ name: 'batch_updated' })
        .eq('name', 'batch_test')
        .select()

      expect(error).toBeNull()
      expect(data!.length).toBe(2)
      expect(data!.every((r) => r.name === 'batch_updated')).toBe(true)

      await supabase.from('instruments').delete().in('id', [9002, 9003])
    })

    it('should handle update to non-existent table with error', async () => {
      const { error } = await supabase
        .from('nonexistent_table')
        .update({ name: 'test' })
        .eq('id', 1)

      expect(error).not.toBeNull()
    })
  })
})
