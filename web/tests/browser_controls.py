"""Drive the visible Radix control, including its actual portal and focus logic."""

import json
from playwright.sync_api import expect


def choose(locator, value):
    expect(locator).to_be_visible()
    locator.click()
    option = locator.page.locator(
        '[role="option"][data-value=' + json.dumps(value) + "]"
    )
    expect(option).to_be_visible()
    option.click()
    expect(locator).to_have_attribute("data-value", value)
    expect(locator).to_be_focused()
