import { chromium } from 'playwright';

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
    emitProgress('info', 'browser', 'Launching browser');
    browser = await chromium.launch({
      headless: false,
      args: ['--start-maximized'],
      // This ensures we can see logs from the browser
      logger: {
        isEnabled: (name) => true,
        log: (name, severity, message) => console.log(`Browser ${severity}: ${message}`)
      }
    });

    emitProgress('info', 'browser', 'Creating browser context');
    context = await browser.newContext({
      viewport: { width: 1920, height: 1080 },
      userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/94.0.4606.81 Safari/537.36'  
    });

    // Enable verbose logging
    context.setDefaultTimeout(60000);
    context.setDefaultNavigationTimeout(60000);

    page = await context.newPage();
    
    emitProgress('info', 'navigation', 'Opening Gitea setup page');
    await page.goto('http://localhost:3000/', {
      waitUntil: 'networkidle',
      timeout: 60000
    });

    // Log the page content for debugging
    const content = await page.content();
    console.log('Page content:', content);

    // Take screenshots at each step
    await page.screenshot({
      path: 'setup-initial.png',
      fullPage: true
    });

    emitProgress('info', 'setup', 'Configuring administrator account');
    await page.click('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/summary', {
      timeout: 10000,
      waitFor: 'visible'
    });

    await page.screenshot({
      path: 'setup-admin-expanded.png',
      fullPage: true
    });

    emitProgress('info', 'setup', 'Filling in administrator details');
    
    // Fill username with delay for visibility
    await page.fill('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/div[1]/input', username);
    await page.waitForTimeout(500);
    
    // Fill email
    await page.fill('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/div[2]/input', email);
    await page.waitForTimeout(500);
    
    // Fill passwords
    await page.fill('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/div[3]/input', password);
    await page.fill('xpath=/html/body/div[1]/div/div/div/div/form/div[15]/details[3]/div[4]/input', password);
    
    await page.screenshot({
      path: 'setup-filled.png',
      fullPage: true
    });

    emitProgress('info', 'setup', 'Starting Gitea installation');
    
    // Click install and wait for completion
    await Promise.all([
      page.waitForNavigation({ waitUntil: 'networkidle', timeout: 120000 }),
      page.click('xpath=/html/body/div[1]/div/div/div/div/form/div[18]/div[2]/button')
    ]);

    await page.screenshot({
      path: 'setup-complete.png',
      fullPage: true
    });

    emitProgress('success', 'setup', 'Installation completed successfully');
    
    return {
      success: true,
      message: 'Gitea installation completed successfully'
    };

  } catch (error) {
    emitProgress('error', 'setup', `Setup failed: ${error.message}`);
    
    if (page) {
      await page.screenshot({
        path: 'setup-error.png',
        fullPage: true
      });

      // Log the page content on error
      const errorContent = await page.content();
      console.error('Page content at error:', errorContent);
    }

    return {
      success: false,
      message: `Setup failed: ${error.message}`
    };

  } finally {
    if (context) {
      await context.close();
    }
    if (browser) {
      emitProgress('info', 'cleanup', 'Closing browser');
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
