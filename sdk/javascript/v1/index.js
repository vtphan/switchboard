/**
 * Switchboard Client SDK - Core Package
 * 
 * Main entry point that exports the core client and non-opinionated helpers.
 * For UI components, import from 'switchboard-client/ui'
 */

// Import the core client (now without built-in helpers)
import SwitchboardClient from './src/client.js';

// Import non-opinionated helpers separately
import * as helpers from './src/helpers.js';

// Export core functionality
export { SwitchboardClient, helpers };
export default SwitchboardClient;