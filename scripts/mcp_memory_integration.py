#!/usr/bin/env python3
"""
MCP Memory Integration
非同期Product Backlogアイテムをmcp-assoc-memoryの共有メモリに保存
"""

import json
from typing import Dict, Any


class BacklogItem:
    """backlog_processorと同等のクラス定義"""
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


class MCPMemoryIntegrator:
    def __init__(self, mcp_server_name: str = "mcp-assoc-mem"):
        self.mcp_server_name = mcp_server_name
    
    def store_backlog_item(self, item: BacklogItem) -> bool:
        """BacklogアイテムをMCP共有メモリに保存"""
        try:
            # 共有メモリ用のタイトルとコンテンツを準備
            memory_title = f"Product Backlog: [{item.priority}] {item.title}"
            memory_content = item.to_memory_content()
            
            # MCPコマンドを構築（実際の実装では適切なMCP APIを使用）
            # この例ではJSON形式でのコマンド実行をシミュレート
            mcp_command = {
                "action": "store_shared_memory",
                "title": memory_title,
                "content": memory_content,
                "creator_persona_id": "context-mdtvs82o-1ekq4d",  # ProductOwner
                "permission_level": "edit",
                "tags": [
                    f"priority:{item.priority.lower()}",
                    "product-backlog",
                    f"beneficiary:{item.beneficiary.lower().replace(' ', '-')}",
                    "sprint-planning"
                ]
            }
            
            # 実際のMCP実行（この例では標準出力に情報を表示）
            print(f"Storing to MCP Memory: {memory_title}")
            print(f"Content Preview: {memory_content[:200]}...")
            print(f"Command: {json.dumps(mcp_command, indent=2)}")
            
            # ここで実際のMCP APIコールを実行
            # result = self._execute_mcp_command(mcp_command)
            
            return True
            
        except Exception as e:
            print(f"Error storing item to MCP memory: {e}")
            return False
    
    def _execute_mcp_command(self, command: Dict[str, Any]) -> Dict[str, Any]:
        """MCP APIコマンドを実行（実装例）"""
        # 実際の実装では、適切なMCP API呼び出しを行う
        # この例ではサンプルレスポンスを返す
        return {
            "success": True,
            "memory_id": f"mem-backlog-{command['title'].replace(' ', '-').lower()}",
            "message": "Successfully stored to shared memory"
        }
    
    def notify_team_members(self, item: BacklogItem) -> bool:
        """チームメンバーに新しいBacklogアイテムを通知"""
        try:
            team_contexts = [
                "context-mdtvs82o-1ekq4d",  # ProductOwner
                "context-mdtvso8o-bljl6h",  # ScrumMaster
                "context-mdtvu1aq-e07dnk",  # TechnicalLead
                "context-mdtvvnz6-exfq23",  # QAEngineer
            ]
            
            notification_message = f"""新しいProduct Backlogアイテムが追加されました:

**[{item.priority}] {item.title}**

**概要**: {item.description}
**受益者**: {item.beneficiary}
**受諾条件**: {item.acceptance_criteria}

次回Sprint Planningでの検討が必要です。"""
            
            for context_id in team_contexts:
                print(f"Notifying {context_id}: {item.title}")
                # 実際の実装では、各コンテキストに通知を送信
                # self._send_notification(context_id, notification_message)
            
            return True
            
        except Exception as e:
            print(f"Error notifying team members: {e}")
            return False
    
    def update_sprint_shared_memory(self, items: list[BacklogItem]) -> bool:
        """Sprint共有メモリを新しいアイテムで更新"""
        try:
            sprint_summary = self._generate_sprint_summary(items)
            
            # Sprint共有メモリ更新（例）
            sprint_memory = {
                "action": "update_shared_memory",
                "title": "Current Sprint - Product Backlog Updates",
                "content": sprint_summary,
                "updater_persona_id": "context-mdtvso8o-bljl6h",  # ScrumMaster
            }
            
            print("Updating Sprint shared memory...")
            print(f"Summary: {sprint_summary[:300]}...")
            
            return True
            
        except Exception as e:
            print(f"Error updating sprint memory: {e}")
            return False
    
    def _generate_sprint_summary(self, items: list[BacklogItem]) -> str:
        """Sprint用のサマリーを生成"""
        if not items:
            return "No new backlog items processed."
        
        priority_counts = {}
        for item in items:
            priority_counts[item.priority] = priority_counts.get(item.priority, 0) + 1
        
        summary = f"""# Sprint Product Backlog Update

## 新規追加アイテム（{len(items)}件）

### 優先度別サマリー
"""
        
        for priority in ["HIGH", "MEDIUM", "LOW"]:
            count = priority_counts.get(priority, 0)
            if count > 0:
                summary += f"- **{priority}**: {count}件\n"
        
        summary += "\n### アイテム詳細\n"
        for item in sorted(items, key=lambda x: ["HIGH", "MEDIUM", "LOW"].index(x.priority)):
            summary += f"""
#### [{item.priority}] {item.title}
- **受益者**: {item.beneficiary}
- **説明**: {item.description}
- **受諾条件**: {item.acceptance_criteria}
"""
        
        summary += """
## Sprint Planning への影響
- 優先度HIGHアイテムは次回Sprint候補
- 依存関係の確認が必要
- 工数見積もりとキャパシティ計画の更新

## 推奨アクション
1. TechnicalLeadによる技術的実現性の評価
2. QAEngineerによるテスト戦略の検討
3. 既存Sprint Backlogとの優先度調整
"""
        
        return summary

# エントリーポイント関数
def integrate_with_mcp(items: list[BacklogItem]) -> bool:
    """処理済みBacklogアイテムをMCPメモリシステムに統合"""
    integrator = MCPMemoryIntegrator()
    
    success_count = 0
    
    # 各アイテムを個別に保存
    for item in items:
        if integrator.store_backlog_item(item):
            success_count += 1
    
    # チームメンバーに通知
    for item in items:
        integrator.notify_team_members(item)
    
    # Sprint共有メモリを更新
    integrator.update_sprint_shared_memory(items)
    
    print(f"MCP Integration completed: {success_count}/{len(items)} items processed")
    
    return success_count == len(items)

if __name__ == "__main__":
    # テスト用のサンプルアイテム
    test_item = BacklogItem(
        priority="HIGH",
        title="Test Backlog Item",
        description="テスト用のBacklogアイテム",
        beneficiary="開発チーム",
        acceptance_criteria="正常に動作すること",
        created_at="2025-08-02 16:10:00"
    )
    
    integrate_with_mcp([test_item])
