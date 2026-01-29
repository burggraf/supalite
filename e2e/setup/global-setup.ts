/**
 * Global test setup for supalite E2E tests
 *
 * This file runs before all tests and sets up the test environment.
 */

import { beforeAll, afterAll } from 'vitest'
import { createClient } from '@supabase/supabase-js'
import * as jwt from 'jsonwebtoken'
import { Client } from 'pg'

// JWT secret for test environment (must match server)
const JWT_SECRET = process.env.SUPALITE_JWT_SECRET || 'super-secret-jwt-token-with-at-least-32-characters-long'

// Generate API keys using the same JWT secret as the server
function generateTestAPIKey(role: string): string {
  return jwt.sign(
    { role, iss: 'supabase', iat: Math.floor(Date.now() / 1000) },
    JWT_SECRET
  )
}

// Test configuration
export const TEST_CONFIG = {
  SUPALITE_URL: process.env.SUPALITE_URL || 'http://localhost:8080',
  SUPALITE_ANON_KEY: process.env.SUPALITE_ANON_KEY || generateTestAPIKey('anon'),
  SUPALITE_SERVICE_KEY: process.env.SUPALITE_SERVICE_KEY || generateTestAPIKey('service_role'),
  JWT_SECRET: JWT_SECRET,
  PG_HOST: process.env.SUPALITE_PG_HOST || 'localhost',
  PG_PORT: parseInt(process.env.SUPALITE_PG_PORT || '5432'),
  PG_DATABASE: process.env.SUPALITE_PG_DATABASE || 'postgres',
  PG_USER: process.env.SUPALITE_PG_USER || 'postgres',
  PG_PASSWORD: process.env.SUPALITE_PG_PASSWORD || 'postgres',
}

// Create Supabase client for testing
export function createTestClient() {
  return createClient(TEST_CONFIG.SUPALITE_URL, TEST_CONFIG.SUPALITE_ANON_KEY, {
    auth: {
      autoRefreshToken: false,
      persistSession: false,
    },
  })
}

// Create authenticated client
export function createAuthenticatedClient(accessToken: string) {
  return createClient(TEST_CONFIG.SUPALITE_URL, TEST_CONFIG.SUPALITE_ANON_KEY, {
    global: {
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
    },
    auth: {
      autoRefreshToken: false,
      persistSession: false,
    },
  })
}

// Setup test database
async function setupTestDatabase() {
  const client = new Client({
    host: TEST_CONFIG.PG_HOST,
    port: TEST_CONFIG.PG_PORT,
    database: TEST_CONFIG.PG_DATABASE,
    user: TEST_CONFIG.PG_USER,
    password: TEST_CONFIG.PG_PASSWORD,
  })

  try {
    await client.connect()
    console.log('   Connected to PostgreSQL')

    // Drop existing test tables if they exist (clean slate)
    console.log('   Dropping existing test tables...')
    const tables = [
      'user_teams',
      'teams',
      'messages',
      'reservations',
      'classes',
      'quotes',
      'issues',
      'users',
      'cities',
      'instruments',
      'orchestral_sections',
      'countries',
      'characters',
      'texts',
      'my table',
      'rls_test',
    ]

    for (const table of tables) {
      try {
        await client.query(`DROP TABLE IF EXISTS "${table}" CASCADE`)
      } catch {
        // Ignore errors
      }
    }

    // Create test tables
    console.log('   Creating test tables...')

    await client.query(`
      CREATE TABLE characters (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        homeworld TEXT,
        is_jedi BOOLEAN
      )
    `)

    await client.query(`
      CREATE TABLE countries (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        code TEXT
      )
    `)

    await client.query(`
      CREATE TABLE orchestral_sections (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL
      )
    `)

    await client.query(`
      CREATE TABLE instruments (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        section_id INTEGER REFERENCES orchestral_sections(id)
      )
    `)

    await client.query(`
      CREATE TABLE cities (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        country_id INTEGER REFERENCES countries(id),
        population INTEGER
      )
    `)

    await client.query(`
      CREATE TABLE users (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        address JSONB
      )
    `)

    await client.query(`
      CREATE TABLE issues (
        id INTEGER PRIMARY KEY,
        title TEXT NOT NULL,
        tags JSONB
      )
    `)

    await client.query(`
      CREATE TABLE quotes (
        id INTEGER PRIMARY KEY,
        catchphrase TEXT NOT NULL
      )
    `)

    await client.query(`
      CREATE TABLE classes (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        days TEXT[]
      )
    `)

    await client.query(`
      CREATE TABLE reservations (
        id INTEGER PRIMARY KEY,
        room TEXT NOT NULL,
        during TSRANGE
      )
    `)

    await client.query(`
      CREATE TABLE messages (
        id INTEGER PRIMARY KEY,
        content TEXT NOT NULL,
        sender_id INTEGER REFERENCES users(id),
        receiver_id INTEGER REFERENCES users(id)
      )
    `)

    await client.query(`
      CREATE TABLE teams (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL
      )
    `)

    await client.query(`
      CREATE TABLE user_teams (
        user_id INTEGER REFERENCES users(id),
        team_id INTEGER REFERENCES teams(id),
        PRIMARY KEY (user_id, team_id)
      )
    `)

    await client.query(`
      CREATE TABLE texts (
        id INTEGER PRIMARY KEY,
        content TEXT NOT NULL
      )
    `)

    await client.query(`
      CREATE TABLE "my table" (
        id INTEGER PRIMARY KEY,
        "my column" TEXT NOT NULL,
        "another column" INTEGER
      )
    `)

    await client.query(`
      CREATE TABLE rls_test (
        id SERIAL PRIMARY KEY,
        user_id TEXT,
        data TEXT
      )
    `)

    // Insert test data
    console.log('   Inserting test data...')

    await client.query(`
      INSERT INTO characters (id, name, homeworld, is_jedi) VALUES
        (1, 'Luke', 'Tatooine', true),
        (2, 'Leia', 'Alderaan', false),
        (3, 'Han', 'Corellia', false),
        (4, 'Yoda', 'Dagobah', true),
        (5, 'Chewbacca', 'Kashyyyk', false)
    `)

    await client.query(`
      INSERT INTO countries (id, name, code) VALUES
        (1, 'United States', 'US'),
        (2, 'Canada', 'CA'),
        (3, 'Mexico', 'MX')
    `)

    await client.query(`
      INSERT INTO orchestral_sections (id, name) VALUES
        (1, 'strings'),
        (2, 'woodwinds'),
        (3, 'percussion')
    `)

    await client.query(`
      INSERT INTO instruments (id, name, section_id) VALUES
        (1, 'violin', 1),
        (2, 'viola', 1),
        (3, 'flute', 2),
        (4, 'clarinet', 2),
        (5, 'piano', 3)
    `)

    await client.query(`
      INSERT INTO cities (id, name, country_id, population) VALUES
        (1, 'New York', 1, 8336817),
        (2, 'Los Angeles', 1, 3979576),
        (3, 'Toronto', 2, 2731571),
        (4, 'Vancouver', 2, 631486),
        (5, 'Smalltown', 1, 5000)
    `)

    await client.query(`
      INSERT INTO users (id, name, address) VALUES
        (1, 'John Doe', '{"street":"123 Main St","city":"New York","postcode":10001}'),
        (2, 'Jane Smith', '{"street":"456 Oak Ave","city":"Beverly Hills","postcode":90210}')
    `)

    await client.query(`
      INSERT INTO issues (id, title, tags) VALUES
        (1, 'Bug: Login fails', '["is:open","priority:high"]'::jsonb),
        (2, 'Feature: Dark mode', '["is:open","priority:low"]'::jsonb),
        (3, 'Bug: Fixed crash', '["is:closed","severity:high"]'::jsonb)
    `)

    await client.query(`
      INSERT INTO quotes (id, catchphrase) VALUES
        (1, 'The quick brown fox jumps over the lazy dog'),
        (2, 'The fat cat sat on the mat'),
        (3, 'A rolling stone gathers no moss')
    `)

    await client.query(`
      INSERT INTO classes (id, name, days) VALUES
        (1, 'Morning Yoga', ARRAY['monday', 'wednesday', 'friday']),
        (2, 'Evening Spin', ARRAY['tuesday', 'thursday']),
        (3, 'Weekend Run', ARRAY['saturday', 'sunday'])
    `)

    await client.query(`
      INSERT INTO reservations (id, room, during) VALUES
        (1, 'A', '[2000-01-01 09:00, 2000-01-01 10:00)'),
        (2, 'B', '[2000-01-01 12:00, 2000-01-01 14:00)'),
        (3, 'A', '[2000-01-02 08:00, 2000-01-02 09:00)')
    `)

    await client.query(`
      INSERT INTO messages (id, content, sender_id, receiver_id) VALUES
        (1, 'Hello!', 1, 2),
        (2, 'Hi there!', 2, 1)
    `)

    await client.query(`
      INSERT INTO teams (id, name) VALUES
        (1, 'Team Alpha'),
        (2, 'Team Beta')
    `)

    await client.query(`
      INSERT INTO user_teams (user_id, team_id) VALUES
        (1, 1),
        (1, 2),
        (2, 1)
    `)

    await client.query(`
      INSERT INTO texts (id, content) VALUES
        (1, 'Green eggs and ham are delicious'),
        (2, 'I do not like them Sam I am'),
        (3, 'Would you eat them in a box')
    `)

    await client.query(`
      INSERT INTO "my table" (id, "my column", "another column") VALUES
        (1, 'first row', 100),
        (2, 'second row', 200),
        (3, 'third row', 300)
    `)

    console.log('   Test database setup complete!')

  } catch (error) {
    console.error('   Failed to setup test database:', error)
    throw error
  } finally {
    await client.end()
  }
}

// Global setup
beforeAll(async () => {
  console.log('\n📦 Setting up E2E test environment...')
  console.log(`   URL: ${TEST_CONFIG.SUPALITE_URL}`)
  console.log(`   PostgreSQL: ${TEST_CONFIG.PG_HOST}:${TEST_CONFIG.PG_PORT}/${TEST_CONFIG.PG_DATABASE}`)

  // Setup test database
  await setupTestDatabase()
})

afterAll(async () => {
  console.log('\n🧹 Cleaning up E2E test environment...')
})
