import puppeteer from 'puppeteer';

function log(status, message) {
  console.log(JSON.stringify({ status, message, timestamp: new Date().toISOString() }));
}

async function uploadSSHKey(username, password, sshKey) {
  let browser;
  let page;
  try {
    log('starting', 'Launching browser');
    browser = await puppeteer.launch({
      headless: false,
      defaultViewport: null,
      args: ['--start-maximized']
    });
    page = await browser.newPage();
    const timeout = 30000;
    page.setDefaultTimeout(timeout);

    try {
      log('setup', 'Setting viewport');
      await page.setViewport({ width: 1280, height: 800 });
    } catch (error) {
      log('error', `Failed to set viewport: ${error.message}`);
      throw error;
    }

    try {
      log('navigating', 'Opening login page');
      await page.goto('http://localhost:3000/user/login', { waitUntil: 'networkidle0' });
      log('page_loaded', 'Login page loaded');
    } catch (error) {
      log('error', `Failed to load login page: ${error.message}`);
      throw error;
    }

    try {
      log('input', 'Entering username');
      await page.waitForSelector('xpath=/html/body/div/div/div/div/div/form/div[1]/input');
      const [usernameInput] = await page.$$('xpath=/html/body/div/div/div/div/div/form/div[1]/input');
      await usernameInput.type(username);

      log('input', 'Entering password');
      await page.waitForSelector('xpath=/html/body/div/div/div/div/div/form/div[2]/input');
      const [passwordInput] = await page.$$('xpath=/html/body/div/div/div/div/div/form/div[2]/input');
      await passwordInput.type(password);
    } catch (error) {
      log('error', `Failed to enter login credentials: ${error.message}`);
      throw error;
    }

    try {
      log('action', 'Clicking login button');
      await page.waitForSelector('xpath=/html/body/div/div/div/div/div/form/div[4]/button');
      const [loginButton] = await page.$$('xpath=/html/body/div/div/div/div/div/form/div[4]/button');
      await Promise.all([
        page.waitForNavigation({ waitUntil: 'networkidle0' }),
        loginButton.evaluate(b => b.click())
      ]);
      log('navigation', 'Logged in successfully');
    } catch (error) {
      log('error', `Failed to click login button or navigate: ${error.message}`);
      throw error;
    }

    try {
      log('navigating', 'Opening SSH Keys page');
      await page.goto('http://localhost:3000/user/settings/keys', { waitUntil: 'networkidle0' });
      log('page_loaded', 'SSH Keys page loaded');
    } catch (error) {
      log('error', `Failed to load SSH Keys page: ${error.message}`);
      throw error;
    }

    try {
      log('action', 'Clicking Add Key button');
      await page.waitForSelector('xpath=/html/body/div[1]/div/div/div[2]/div/h4[1]/div/button');
      const [addKeyButton] = await page.$$('xpath=/html/body/div[1]/div/div/div[2]/div/h4[1]/div/button');
      await addKeyButton.evaluate(b => b.click());
      log('ui_change', 'Add Key form opened');
    } catch (error) {
      log('error', `Failed to open Add Key form: ${error.message}`);
      throw error;
    }

    const keyTitle = `Gitspace Generated Key (${new Date().toISOString()})`;
    try {
      log('input', `Entering key title: ${keyTitle}`);
      await page.waitForSelector('xpath=/html/body/div[1]/div/div/div[2]/div/div[1]/div[1]/form/div[1]/input');
      const [keyNameInput] = await page.$$('xpath=/html/body/div[1]/div/div/div[2]/div/div[1]/div[1]/form/div[1]/input');
      await keyNameInput.type(keyTitle);

      log('input', 'Pasting SSH key');
      await page.waitForSelector('xpath=/html/body/div[1]/div/div/div[2]/div/div[1]/div[1]/form/div[2]/textarea');
      const [keyContentTextarea] = await page.$$('xpath=/html/body/div[1]/div/div/div[2]/div/div[1]/div[1]/form/div[2]/textarea');
      await keyContentTextarea.type(sshKey);
    } catch (error) {
      log('error', `Failed to enter key details: ${error.message}`);
      throw error;
    }

    try {
      log('action', 'Submitting SSH key form');
      await page.waitForSelector('xpath=/html/body/div[1]/div/div/div[2]/div/div[1]/div[1]/form/button[1]');
      const [submitButton] = await page.$$('xpath=/html/body/div[1]/div/div/div[2]/div/div[1]/div[1]/form/button[1]');
      await Promise.all([
        page.waitForNavigation({ waitUntil: 'networkidle0' }),
        submitButton.evaluate(b => b.click())
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
