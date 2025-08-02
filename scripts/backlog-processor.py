#!/usr/bin/env python3
"""
Product Backlog Processor
非同期でファイルから新しいBacklogアイテムを読み取り、
mcp-assoc-memoryの共有メモリに保存するスクリプト
"""

import re
import os
import json
from datetime import datetime
from typing import List, Dict, Optional

class BacklogItem:
    def __init__(self, priority: str, title: str, description: str, 
                 beneficiary: str, acceptance_criteria: str, 
                 created_at: str, status: str = "NEW"):
        self.priority = priority
        self.title = title
        self.description = description
        self.beneficiary = beneficiary
        self.acceptance_criteria = acceptance_criteria
        self.created_at = created_at
        self.status = status
        self.processed_at = None
    
    def to_dict(self) -> Dict:
        return {
            "priority": self.priority,
            "title": self.title,
            "description": self.description,
            "beneficiary": self.beneficiary,
            "acceptance_criteria": self.acceptance_criteria,
            "created_at": self.created_at,
            "status": self.status,
            "processed_at": self.processed_at
        }
    
    def to_memory_content(self) -> str:
        """共有メモリ用のフォーマット"""
        return f"""# Product Backlog Item: {self.title}

## 詳細情報
- **優先度**: {self.priority}
- **タイトル**: {self.title}
- **説明**: {self.description}
- **受益者**: {self.beneficiary}
- **受諾条件**: {self.acceptance_criteria}
- **作成日時**: {self.created_at}
- **ステータス**: {self.status}
- **処理日時**: {self.processed_at or "未処理"}

## Sprint計画への影響
- 優先度 {self.priority} として、次回Sprint Planningで検討
- {self.beneficiary}への価値提供を考慮した実装計画が必要

## 技術的考慮事項
- 受諾条件: {self.acceptance_criteria}
- 既存バックログとの依存関係確認が必要
"""

class BacklogProcessor:
    def __init__(self, backlog_file: str):
        self.backlog_file = backlog_file
        self.processed_items: List[BacklogItem] = []
    
    def parse_backlog_file(self) -> List[BacklogItem]:
        """Backlogファイルから新しいアイテムを解析"""
        if not os.path.exists(self.backlog_file):
            return []
        
        with open(self.backlog_file, 'r', encoding='utf-8') as f:
            content = f.read()
        
        # 正規表現でBacklogアイテムを抽出
        pattern = r'### \[(\w+)\] (.+?)\n- \*\*説明\*\*: (.+?)\n- \*\*受益者\*\*: (.+?)\n- \*\*受諾条件\*\*: (.+?)\n- \*\*作成日時\*\*: (.+?)\n- \*\*ステータス\*\*: (\w+)'
        
        matches = re.findall(pattern, content, re.MULTILINE | re.DOTALL)
        
        items = []
        for match in matches:
            priority, title, description, beneficiary, acceptance_criteria, created_at, status = match
            
            # NEWステータスのアイテムのみ処理
            if status.strip() == "NEW":
                item = BacklogItem(
                    priority=priority.strip(),
                    title=title.strip(),
                    description=description.strip(),
                    beneficiary=beneficiary.strip(),
                    acceptance_criteria=acceptance_criteria.strip(),
                    created_at=created_at.strip(),
                    status=status.strip()
                )
                items.append(item)
        
        return items
    
    def process_items(self, items: List[BacklogItem]) -> int:
        """アイテムを処理し、共有メモリに保存"""
        processed_count = 0
        
        for item in items:
            try:
                # 処理時刻を設定
                item.processed_at = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
                item.status = "PROCESSED"
                
                # 共有メモリ保存用のデータ準備
                memory_content = item.to_memory_content()
                
                # MCP統合を試行
                try:
                    from mcp_memory_integration import integrate_with_mcp
                    integrate_with_mcp([item])
                    print(f"Integrated with MCP: [{item.priority}] {item.title}")
                except ImportError:
                    print(f"MCP integration not available for: {item.title}")
                except Exception as mcp_error:
                    print(f"MCP integration failed for {item.title}: {mcp_error}")
                
                # 処理済みアイテムをリストに追加
                self.processed_items.append(item)
                processed_count += 1
                
                print(f"Processed: [{item.priority}] {item.title}")
                
            except Exception as e:
                print(f"Error processing item {item.title}: {e}")
        
        return processed_count
    
    def update_backlog_file(self, items: List[BacklogItem]):
        """処理済みアイテムのステータスをファイルに反映"""
        if not os.path.exists(self.backlog_file):
            return
        
        with open(self.backlog_file, 'r', encoding='utf-8') as f:
            content = f.read()
        
        # 処理済みアイテムのステータスを更新
        for item in items:
            if item.status == "PROCESSED":
                old_status = f"**ステータス**: NEW"
                new_status = f"**ステータス**: PROCESSED ({item.processed_at})"
                content = content.replace(old_status, new_status)
        
        # Copilot処理ログを更新
        log_pattern = r'- \*\*最終確認日時\*\*: .+\n- \*\*処理済みアイテム数\*\*: \d+\n- \*\*未処理アイテム数\*\*: \d+'
        
        current_time = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        processed_count = len([item for item in self.processed_items if item.status == "PROCESSED"])
        new_items_count = content.count("**ステータス**: NEW")
        
        new_log = f"""- **最終確認日時**: {current_time}
- **処理済みアイテム数**: {processed_count}
- **未処理アイテム数**: {new_items_count}"""
        
        content = re.sub(log_pattern, new_log, content)
        
        with open(self.backlog_file, 'w', encoding='utf-8') as f:
            f.write(content)
    
    def generate_summary_report(self) -> str:
        """処理サマリーレポートを生成"""
        if not self.processed_items:
            return "No new items processed."
        
        priority_counts = {}
        for item in self.processed_items:
            priority_counts[item.priority] = priority_counts.get(item.priority, 0) + 1
        
        report = f"""# Product Backlog Processing Report

## 処理サマリー
- **処理日時**: {datetime.now().strftime("%Y-%m-%d %H:%M:%S")}
- **処理済みアイテム数**: {len(self.processed_items)}

## 優先度別内訳
"""
        
        for priority, count in sorted(priority_counts.items()):
            report += f"- **{priority}**: {count}件\n"
        
        report += "\n## 処理済みアイテム詳細\n"
        for item in self.processed_items:
            report += f"- [{item.priority}] {item.title}\n"
        
        return report

def main():
    backlog_file = "/home/mako10k/llmcmd/product-backlog-requests.md"
    processor = BacklogProcessor(backlog_file)
    
    print("Starting Product Backlog processing...")
    
    # 新しいアイテムを解析
    new_items = processor.parse_backlog_file()
    
    if not new_items:
        print("No new items found.")
        return
    
    print(f"Found {len(new_items)} new items:")
    for item in new_items:
        print(f"  - [{item.priority}] {item.title}")
    
    # アイテムを処理
    processed_count = processor.process_items(new_items)
    
    # ファイルを更新
    processor.update_backlog_file(new_items)
    
    # サマリーレポート生成
    report = processor.generate_summary_report()
    print("\n" + report)
    
    # レポートファイルに保存
    report_file = f"/home/mako10k/llmcmd/backlog-processing-{datetime.now().strftime('%Y%m%d-%H%M%S')}.md"
    with open(report_file, 'w', encoding='utf-8') as f:
        f.write(report)
    
    print(f"\nReport saved to: {report_file}")

if __name__ == "__main__":
    main()
