import { chromium } from 'playwright';

function log(status, message) {
  console.log(JSON.stringify({ status, message, timestamp: new Date().toISOString() }));
}

async function uploadSSHKey(username, password, sshKey) {
  let browser;
  let page;
  try {
    log('starting', 'Launching browser');
    browser = await chromium.launch({ headless: false });
    page = await browser.newPage();

    try {
      log('navigating', 'Opening login page');
      await page.goto('http://localhost:3000/user/login', { waitUntil: 'networkidle' });
      log('page_loaded', 'Login page loaded');
    } catch (error) {
      log('error', `Failed to load login page: ${error.message}`);
      throw error;
    }

    try {
      log('input', 'Entering username');
      await page.fill('xpath=/html/body/div/div/div/div/div/form/div[1]/input', username);

      log('input', 'Entering password');
      await page.fill('xpath=/html/body/div/div/div/div/div/form/div[2]/input', password);

      log('action', 'Clicking login button');
      await Promise.all([
        page.waitForNavigation({ waitUntil: 'networkidle' }),
        page.click('xpath=/html/body/div/div/div/div/div/form/div[4]/button')
      ]);
      log('navigation', 'Logged in successfully');
    } catch (error) {
      log('error', `Failed to login: ${error.message}`);
      throw error;
    }

    try {
      log('navigating', 'Opening SSH Keys page');
      await page.goto('http://localhost:3000/user/settings/keys', { waitUntil: 'networkidle' });
      log('page_loaded', 'SSH Keys page loaded');
    } catch (error) {
      log('error', `Failed to load SSH Keys page: ${error.message}`);
      throw error;
    }

    try {
      log('action', 'Clicking Add Key button');
      await page.click('xpath=/html/body/div[1]/div/div/div[2]/div/h4[1]/div/button');
      log('ui_change', 'Add Key form opened');
    } catch (error) {
      log('error', `Failed to open Add Key form: ${error.message}`);
      throw error;
    }

    const keyTitle = `Gitspace Generated Key (${new Date().toISOString()})`;
    try {
      log('input', `Entering key title: ${keyTitle}`);
      await page.fill('xpath=/html/body/div[1]/div/div/div[2]/div/div[1]/div[1]/form/div[1]/input', keyTitle);
      
      log('input', 'Pasting SSH key');
      await page.fill('xpath=/html/body/div[1]/div/div/div[2]/div/div[1]/div[1]/form/div[2]/textarea', sshKey);
    } catch (error) {
      log('error', `Failed to enter key details: ${error.message}`);
      throw error;
    }

    try {
      log('action', 'Submitting SSH key form');
      await Promise.all([
        page.waitForNavigation({ waitUntil: 'networkidle' }),
        page.click('xpath=/html/body/div[1]/div/div/div[2]/div/div[1]/div[1]/form/button[1]')
      ]);
    } catch (error) {
      log('error', `Failed to submit SSH key form: ${error.message}`);
      throw error;
    }

    try {
      log('verifying', 'Checking for success message');
      await page.waitForSelector('xpath=//div[contains(@class, "ui") and contains(@class, "positive") and contains(@class, "message")]', { timeout: 5000 });
      log('success', 'SSH key uploaded successfully');
    } catch (error) {
      log('error', `Failed to verify success message: ${error.message}`);
      throw error;
    }

    try {
      await page.screenshot({ path: 'success_screenshot.png', fullPage: true });
      log('artifact', 'Saved success screenshot');
    } catch (error) {
      log('error', `Failed to save success screenshot: ${error.message}`);
    }

  } catch (error) {
    log('error', `Error during SSH key upload process: ${error.message}`);
    if (page) {
      try {
        await page.screenshot({ path: 'error_screenshot.png', fullPage: true });
        log('artifact', 'Saved error screenshot');
      } catch (screenshotError) {
        log('error', `Failed to save error screenshot: ${screenshotError.message}`);
      }
    }
    throw error;
  } finally {
    if (browser) {
      log('cleanup', 'Closing browser');
      await browser.close();
    }
  }
}

// Main execution
const [, , username, password, sshKey] = process.argv;

if (!username || !password || !sshKey) {
  log('error', 'Missing required arguments. Usage: node script.mjs <username> <password> "<ssh-key>"');
  process.exit(1);
}

uploadSSHKey(username, password, sshKey)
  .then(() => {
    log('complete', 'Script execution completed successfully');
    process.exit(0);
  })
  .catch(error => {
    log('fatal', `Unhandled error: ${error.message}`);
    process.exit(1);
  });

export { uploadSSHKey };
