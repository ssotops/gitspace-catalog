import { chromium } from 'playwright';

// Helper to send structured progress messages to stdout
function emitProgress(status, step, details = '') {
  const message = JSON.stringify({
    type: 'progress',
    payload: {
      status,
      step,
      details,
      timestamp: new Date().toISOString()
    }
  });
  console.log(message);
}

async function setupGitea(username, email, password) {
  let browser;
  let context;
  let page;

  try {
    emitProgress('started', 'browser', 'Launching browser');
    browser = await chromium.launch({
      headless: false,
      args: ['--start-maximized']
    });

    // Create context with larger viewport
    context = await browser.newContext({
      viewport: { width: 1920, height: 1080 },
      recordVideo: { dir: 'videos/' }
    });

    page = await context.newPage();
    
    emitProgress('started', 'navigation', 'Loading Gitea initial setup page');
    await page.goto('http://localhost:3000/', {
      waitUntil: 'networkidle',
      timeout: 60000
    });
    emitProgress('completed', 'navigation', 'Initial page loaded');

    emitProgress('started', 'setup', 'Opening administrator settings');
    await page.click('div.ui.container form details >> text=Admin Account Settings');
    emitProgress('completed', 'setup', 'Administrator section expanded');

    // Fill in administrator account details with pauses for visibility
    emitProgress('started', 'input', 'Entering administrator details');
    
    await page.fill('input[name="admin_name"]', username);
    emitProgress('progress', 'input', 'Username entered');
    await page.waitForTimeout(500);

    await page.fill('input[name="admin_email"]', email);
    emitProgress('progress', 'input', 'Email entered');
    await page.waitForTimeout(500);

    await page.fill('input[name="admin_password"]', password);
    await page.fill('input[name="admin_confirm_password"]', password);
    emitProgress('progress', 'input', 'Password configured');
    await page.waitForTimeout(500);

    emitProgress('started', 'installation', 'Starting Gitea installation');
    
    // Click install and wait for completion
    const [response] = await Promise.all([
      page.waitForNavigation({
        waitUntil: 'networkidle',
        timeout: 120000
      }),
      page.click('button >> text=Install Gitea')
    ]);

    // Take a screenshot of the completed installation
    await page.screenshot({ 
      path: 'gitea-setup-complete.png',
      fullPage: true 
    });

    emitProgress('completed', 'installation', 'Gitea installation completed successfully');
    
    return {
      success: true,
      message: 'Gitea installation completed successfully',
      screenshot: 'gitea-setup-complete.png'
    };

  } catch (error) {
    emitProgress('error', 'setup', `Setup failed: ${error.message}`);
    
    if (page) {
      await page.screenshot({ 
        path: 'gitea-setup-error.png',
        fullPage: true 
      });
    }

    return {
      success: false,
      message: `Setup failed: ${error.message}`,
      screenshot: 'gitea-setup-error.png'
    };

  } finally {
    if (context) {
      await context.close();
    }
    if (browser) {
      emitProgress('cleanup', 'browser', 'Closing browser');
      await browser.close();
    }
  }
}

// Main execution
const [, , username, email, password] = process.argv;

if (!username || !email || !password) {
  console.error(JSON.stringify({
    type: 'error',
    payload: {
      message: 'Missing required arguments. Usage: node setup_gitea.js <username> <email> <password>'
    }
  }));
  process.exit(1);
}

setupGitea(username, email, password)
  .then(result => {
    console.log(JSON.stringify({
      type: 'result',
      payload: result
    }));
    process.exit(result.success ? 0 : 1);
  })
  .catch(error => {
    console.error(JSON.stringify({
      type: 'error',
      payload: {
        message: `Unhandled error: ${error.message}`
      }
    }));
    process.exit(1);
  });
