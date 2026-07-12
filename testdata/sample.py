#!/usr/bin/env python3
"""Sample Python file for testing syntax highlighting."""

import json
import sys
from dataclasses import dataclass
from typing import List, Optional


@dataclass
class Item:
    """A shopping list item."""
    name: str
    quantity: int = 1
    purchased: bool = False


class ShoppingList:
    """Manage a shopping list."""

    def __init__(self, title: str = "Grocery List"):
        self.title = title
        self.items: List[Item] = []

    def add_item(self, name: str, quantity: int = 1) -> Item:
        item = Item(name=name, quantity=quantity)
        self.items.append(item)
        return item

    def mark_purchased(self, name: str) -> bool:
        for item in self.items:
            if item.name == name:
                item.purchased = True
                return True
        return False

    def to_json(self) -> str:
        return json.dumps([
            {"name": i.name, "qty": i.quantity, "done": i.purchased}
            for i in self.items
        ], indent=2)


def main():
    shopping = ShoppingList("Weekend Groceries")
    shopping.add_item("Milk", 2)
    shopping.add_item("Bread")
    shopping.add_item("Eggs", 12)
    shopping.add_item("Apples", 6)

    shopping.mark_purchased("Milk")
    shopping.mark_purchased("Bread")

    print(f"Shopping List: {shopping.title}")
    for item in shopping.items:
        status = "✓" if item.purchased else " "
        print(f"  [{status}] {item.name} (x{item.quantity})")

    print("\nJSON Output:")
    print(shopping.to_json())


if __name__ == "__main__":
    main()
