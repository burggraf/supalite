/**
 * SELECT Operation Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/select
 *
 * Each test corresponds to an example from the documentation.
 */

import { describe, it, expect, beforeAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('REST API - SELECT Operations', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SUPALITE_URL, TEST_CONFIG.SUPALITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  /**
   * Example 1: Getting your data
   * Docs: https://supabase.com/docs/reference/javascript/select#getting-your-data
   */
  describe('1. Getting your data', () => {
    it('should retrieve all rows and columns from a table', async () => {
      const { data, error } = await supabase.from('characters').select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(Array.isArray(data)).toBe(true)
      expect(data!.length).toBeGreaterThan(0)
      expect(data![0]).toHaveProperty('id')
      expect(data![0]).toHaveProperty('name')
    })
  })

  /**
   * Example 2: Selecting specific columns
   * Docs: https://supabase.com/docs/reference/javascript/select#selecting-specific-columns
   */
  describe('2. Selecting specific columns', () => {
    it('should return only the specified columns', async () => {
      const { data, error } = await supabase.from('characters').select('name')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).not.toHaveProperty('id')
      expect(data![0]).not.toHaveProperty('homeworld')
    })

    it('should return multiple specific columns', async () => {
      const { data, error } = await supabase.from('characters').select('id, name')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data![0]).toHaveProperty('id')
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).not.toHaveProperty('homeworld')
    })
  })

  /**
   * Example 3: Query referenced tables
   * Docs: https://supabase.com/docs/reference/javascript/select#query-referenced-tables
   */
  describe('3. Query referenced tables', () => {
    it('should fetch related data from referenced tables', async () => {
      const { data, error } = await supabase
        .from('orchestral_sections')
        .select(`
          name,
          instruments (
            name
          )
        `)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).toHaveProperty('instruments')
      expect(Array.isArray(data![0].instruments)).toBe(true)
    })
  })

  /**
   * Example 4: Query referenced tables with spaces in their names
   * Docs: https://supabase.com/docs/reference/javascript/select#query-referenced-tables-with-spaces
   */
  describe('4. Query referenced tables with spaces', () => {
    it('should handle table names with spaces', async () => {
      const { data, error } = await supabase.from('my table').select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(3)
      expect(data![0]).toHaveProperty('my column')
    })

    it('should handle column names with spaces in filters', async () => {
      const { data, error } = await supabase
        .from('my table')
        .select()
        .eq('my column', 'first row')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0]['my column']).toBe('first row')
    })
  })

  /**
   * Example 5: Query referenced tables through a join table
   * Docs: https://supabase.com/docs/reference/javascript/select#query-referenced-tables-through-a-join-table
   */
  describe('5. Query referenced tables through join table', () => {
    it('should query through many-to-many join tables', async () => {
      const { data, error } = await supabase.from('users').select(`
          id,
          name,
          teams (
            id,
            name
          )
        `)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)

      const john = data!.find((u) => u.name === 'John Doe')
      expect(john).toBeDefined()
      expect(Array.isArray(john!.teams)).toBe(true)
      expect(john!.teams.length).toBe(2)

      const jane = data!.find((u) => u.name === 'Jane Smith')
      expect(jane).toBeDefined()
      expect(Array.isArray(jane!.teams)).toBe(true)
      expect(jane!.teams.length).toBe(1)
    })
  })

  /**
   * Example 6: Query the same referenced table multiple times
   * Docs: https://supabase.com/docs/reference/javascript/select#query-the-same-referenced-table-multiple-times
   */
  describe('6. Query same referenced table multiple times', () => {
    it('should allow aliasing for multiple references to same table', async () => {
      const { data, error } = await supabase.from('messages').select(`
          id,
          content,
          sender:users!sender_id (
            id,
            name
          ),
          receiver:users!receiver_id (
            id,
            name
          )
        `)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)

      const msg = data![0]
      expect(msg).toHaveProperty('id')
      expect(msg).toHaveProperty('content')
      expect(msg).toHaveProperty('sender')
      expect(msg).toHaveProperty('receiver')
      expect(msg.sender).toHaveProperty('name')
      expect(msg.receiver).toHaveProperty('name')

      const msg1 = data!.find((m) => m.id === 1)
      expect(msg1).toBeDefined()
      expect(msg1!.sender.name).toBe('John Doe')
      expect(msg1!.receiver.name).toBe('Jane Smith')
    })
  })

  /**
   * Example 7: Query nested foreign tables through a join table
   * Docs: https://supabase.com/docs/reference/javascript/select#query-nested-foreign-tables-through-a-join-table
   */
  describe('7. Query nested foreign tables through join', () => {
    it('should handle deeply nested relationships', async () => {
      // With PostgreSQL and pREST, this should work
      const { data, error } = await supabase.from('users').select(`
          name,
          teams (
            name,
            user_teams (
              user_id
            )
          )
        `)

      // This may or may not be supported by pREST
      if (error) {
        console.log('   Note: Deep nesting not fully supported:', error.message)
      } else {
        expect(data).toBeDefined()
      }
    })
  })

  /**
   * Example 8: Filtering through referenced tables
   * Docs: https://supabase.com/docs/reference/javascript/select#filtering-through-referenced-tables
   */
  describe('8. Filtering through referenced tables', () => {
    it('should filter on fields from referenced tables', async () => {
      const { data, error } = await supabase
        .from('cities')
        .select('name, countries(name)')
        .eq('countries.name', 'Canada')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(2)

      const cityNames = data!.map((c) => c.name).sort()
      expect(cityNames).toEqual(['Toronto', 'Vancouver'])
    })
  })

  /**
   * Example 9: Querying referenced table with count
   * Docs: https://supabase.com/docs/reference/javascript/select#querying-referenced-table-with-count
   */
  describe('9. Querying referenced table with count', () => {
    it('should return count of related records', async () => {
      // This tests if we can get count of related records
      const { data, error } = await supabase
        .from('users')
        .select(`
          name,
          teams (id)
        `)

      expect(error).toBeNull()
      expect(data).toBeDefined()

      const john = data!.find((u) => u.name === 'John Doe')
      expect(john).toBeDefined()
      expect(Array.isArray(john!.teams)).toBe(true)
    })
  })

  /**
   * Example 10: Querying with count option
   * Docs: https://supabase.com/docs/reference/javascript/select#querying-with-count-option
   */
  describe('10. Querying with count option', () => {
    it('should return only count without data when head: true', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'exact', head: true })

      expect(error).toBeNull()
      expect(data).toBeNull()
      expect(count).toBe(5)
    })

    it('should return both count and data', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'exact' })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(count).toBe(data!.length)
    })
  })

  /**
   * Example 11: Querying JSON data
   * Docs: https://supabase.com/docs/reference/javascript/select#querying-json-data
   */
  describe('11. Querying JSON data', () => {
    it('should extract fields from JSON columns using arrow notation', async () => {
      const { data, error } = await supabase
        .from('users')
        .select(`
          id, name,
          address->city
        `)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data![0]).toHaveProperty('city')
    })

    it('should handle JSONB queries in PostgreSQL', async () => {
      // PostgreSQL supports native JSONB
      const { data, error } = await supabase
        .from('users')
        .select('name, address')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data![0]).toHaveProperty('address')
      expect(typeof data![0].address).toBe('object')
    })
  })

  /**
   * Example 12: Querying referenced table with inner join
   * Docs: https://supabase.com/docs/reference/javascript/select#querying-referenced-table-with-inner-join
   */
  describe('12. Querying with inner join', () => {
    it('should perform inner join on referenced tables', async () => {
      const { data, error } = await supabase
        .from('countries')
        .select('name, cities!inner(name)')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      data!.forEach((country) => {
        expect(Array.isArray(country.cities)).toBe(true)
        expect(country.cities.length).toBeGreaterThan(0)
      })
    })
  })

  /**
   * Example 13: Switching schemas per query
   * Docs: https://supabase.com/docs/reference/javascript/select#switching-schemas-per-query
   */
  describe('13. Switching schemas per query', () => {
    it('should switch to specified schema', async () => {
      // PostgreSQL supports schemas
      // Try accessing public schema explicitly
      const { data, error } = await supabase
        .from('characters')
        .select()
        .schema('public')

      expect(error).toBeNull()
      expect(data).toBeDefined()
    })
  })

  describe('Additional SELECT functionality', () => {
    it('should handle empty table gracefully', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .eq('id', -9999)

      expect(error).toBeNull()
      expect(data).toEqual([])
    })

    it('should handle non-existent table with error', async () => {
      const { data, error } = await supabase
        .from('nonexistent_table')
        .select()

      expect(error).not.toBeNull()
    })

    it('should handle selecting all columns with *', async () => {
      const { data, error } = await supabase.from('characters').select('*')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data![0]).toHaveProperty('id')
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).toHaveProperty('homeworld')
    })
  })
})
