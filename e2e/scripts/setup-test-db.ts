#!/usr/bin/env tsx
/**
 * Setup PostgreSQL test database with sample data for E2E tests
 *
 * Run: npx tsx scripts/setup-test-db.ts
 *
 * This script connects to the running PostgreSQL instance and creates
 * test tables with sample data for PostgREST/pREST compatibility testing.
 */

import { Client } from 'pg'

const PG_HOST = process.env.SUPALITE_PG_HOST || 'localhost'
const PG_PORT = parseInt(process.env.SUPALITE_PG_PORT || '5432')
const PG_DATABASE = process.env.SUPALITE_PG_DATABASE || 'postgres'
const PG_USER = process.env.SUPALITE_PG_USER || 'postgres'
const PG_PASSWORD = process.env.SUPALITE_PG_PASSWORD || 'postgres'

console.log('🔧 Setting up PostgreSQL test database...')
console.log(`   Host: ${PG_HOST}:${PG_PORT}`)
console.log(`   Database: ${PG_DATABASE}`)

async function setupDatabase() {
  const client = new Client({
    host: PG_HOST,
    port: PG_PORT,
    database: PG_DATABASE,
    user: PG_USER,
    password: PG_PASSWORD,
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
      'my table',  // Quoted identifier
      'rls_test',
    ]

    for (const table of tables) {
      try {
        await client.query(`DROP TABLE IF EXISTS "${table}" CASCADE`)
      } catch (e) {
        // Ignore errors
      }
    }

    // Create test tables
    console.log('   Creating test tables...')

    // Characters table (Star Wars theme from Supabase docs)
    await client.query(`
      CREATE TABLE characters (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        homeworld TEXT,
        is_jedi BOOLEAN
      )
    `)

    // Countries table
    await client.query(`
      CREATE TABLE countries (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        code TEXT
      )
    `)

    // Orchestral sections table
    await client.query(`
      CREATE TABLE orchestral_sections (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL
      )
    `)

    // Instruments table
    await client.query(`
      CREATE TABLE instruments (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        section_id INTEGER REFERENCES orchestral_sections(id)
      )
    `)

    // Cities table
    await client.query(`
      CREATE TABLE cities (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        country_id INTEGER REFERENCES countries(id),
        population INTEGER
      )
    `)

    // Users table (with JSONB support)
    await client.query(`
      CREATE TABLE users (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        address JSONB
      )
    `)

    // Issues table (with array support via JSONB)
    await client.query(`
      CREATE TABLE issues (
        id INTEGER PRIMARY KEY,
        title TEXT NOT NULL,
        tags JSONB
      )
    `)

    // Quotes table (for text search)
    await client.query(`
      CREATE TABLE quotes (
        id INTEGER PRIMARY KEY,
        catchphrase TEXT NOT NULL
      )
    `)

    // Classes table (for containedBy tests)
    await client.query(`
      CREATE TABLE classes (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        days TEXT[]
      )
    `)

    // Reservations table (for range tests)
    await client.query(`
      CREATE TABLE reservations (
        id INTEGER PRIMARY KEY,
        room TEXT NOT NULL,
        during TSRANGE
      )
    `)

    // Messages table (for self-join / aliased join testing)
    await client.query(`
      CREATE TABLE messages (
        id INTEGER PRIMARY KEY,
        content TEXT NOT NULL,
        sender_id INTEGER REFERENCES users(id),
        receiver_id INTEGER REFERENCES users(id)
      )
    `)

    // Teams table (for M2M testing)
    await client.query(`
      CREATE TABLE teams (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL
      )
    `)

    // User-Teams junction table (for M2M testing)
    await client.query(`
      CREATE TABLE user_teams (
        user_id INTEGER REFERENCES users(id),
        team_id INTEGER REFERENCES teams(id),
        PRIMARY KEY (user_id, team_id)
      )
    `)

    // Texts table (for full-text search)
    await client.query(`
      CREATE TABLE texts (
        id INTEGER PRIMARY KEY,
        content TEXT NOT NULL
      )
    `)

    // Table with spaces in name (for quoted identifier tests)
    await client.query(`
      CREATE TABLE "my table" (
        id INTEGER PRIMARY KEY,
        "my column" TEXT NOT NULL,
        "another column" INTEGER
      )
    `)

    // RLS test table
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

    console.log('✅ Test database setup complete!')
    console.log(`
Test tables created:
   - characters (5 rows)
   - countries (3 rows)
   - orchestral_sections (3 rows)
   - instruments (5 rows)
   - cities (5 rows)
   - users (2 rows)
   - issues (3 rows)
   - quotes (3 rows)
   - classes (3 rows)
   - reservations (3 rows)
   - messages (2 rows)
   - teams (2 rows)
   - user_teams (3 rows)
   - texts (3 rows)
   - rls_test (0 rows)
   - "my table" (3 rows, for quoted identifier tests)

Ready to run tests!
    `)

  } catch (error) {
    console.error('❌ Failed to setup test database:', error)
    process.exit(1)
  } finally {
    await client.end()
  }
}

setupDatabase()
